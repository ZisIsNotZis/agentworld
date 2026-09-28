package checkpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func testSections() []Section {
	return []Section{
		{Journal, 1, []byte("attempts")},
		{KernelHistory, 1, []byte("events")},
		{ManifestLineage, 1, []byte("ancestry")},
		{Scheduler, 1, []byte("wakes")},
		{Strategy, 1, []byte("policies")},
	}
}

func TestBundleCanonicalRoundTrip(t *testing.T) {
	sections := testSections()
	bundle, digest, err := Encode(sections)
	if err != nil {
		t.Fatal(err)
	}
	got, verified, err := Decode(bundle)
	if err != nil || verified != digest || !reflect.DeepEqual(got, sections) {
		t.Fatalf("decoded %v, digest %x, error %v", got, verified, err)
	}
	got[0].Data[0] ^= 1 // returned data aliases the supplied buffer
	if _, _, err := Decode(bundle); !errors.Is(err, ErrBundle) {
		t.Fatalf("mutation must invalidate digest: %v", err)
	}
	for _, index := range []int{0, 1, 2, 3, 4} {
		t.Run(sectionNames[index], func(t *testing.T) {
			bad := testSections()
			bad[index].Data = nil
			if _, _, err := Encode(bad); !errors.Is(err, ErrBundle) {
				t.Fatalf("missing section accepted: %v", err)
			}
		})
	}
}

// rawBundle bypasses Encode so checksum-correct but noncanonical frames can
// exercise the decoder independently of the encoder's validation.
func rawBundle(sections []Section) []byte {
	out := append([]byte(nil), bundleMagic[:]...)
	out = binary.BigEndian.AppendUint32(out, FormatVersion)
	out = binary.BigEndian.AppendUint16(out, uint16(len(sections)))
	for _, s := range sections {
		out = append(out, byte(len(s.Name)))
		out = append(out, s.Name...)
		out = binary.BigEndian.AppendUint32(out, s.Version)
		out = binary.BigEndian.AppendUint32(out, uint32(len(s.Data)))
		out = append(out, s.Data...)
	}
	return checksum(out)
}

func checksum(body []byte) []byte {
	hash := sha256.Sum256(body)
	return append(body, hash[:]...)
}

func TestBundleRejectsMalformedFrames(t *testing.T) {
	good := rawBundle(testSections())
	wrongMagic := bytes.Clone(good)
	wrongMagic[0] ^= 1
	wrongVersion := bytes.Clone(good)
	binary.BigEndian.PutUint32(wrongVersion[4:], 2)
	wrongCount := bytes.Clone(good)
	binary.BigEndian.PutUint16(wrongCount[8:], 255)
	wrongSectionVersion := testSections()
	wrongSectionVersion[0].Version = 2
	unknown := testSections()
	unknown[0].Name = "unknown"
	duplicate := testSections()
	duplicate[1] = duplicate[0]
	unsorted := testSections()
	unsorted[0], unsorted[1] = unsorted[1], unsorted[0]
	tooLong := bytes.Clone(good[:len(good)-sha256.Size])
	binary.BigEndian.PutUint32(tooLong[bundleHeaderSize+1+len(Journal)+4:], uint32(sectionLimits[0]+1))
	cases := map[string][]byte{
		"empty": nil, "truncated header": good[:8], "truncated section": good[:len(good)-40],
		"truncated digest": good[:len(good)-1], "corrupted digest": append(bytes.Clone(good), 0),
		"magic":           checksum(wrongMagic[:len(wrongMagic)-sha256.Size]),
		"version":         checksum(wrongVersion[:len(wrongVersion)-sha256.Size]),
		"count":           checksum(wrongCount[:len(wrongCount)-sha256.Size]),
		"section version": rawBundle(wrongSectionVersion), "unknown section": rawBundle(unknown),
		"duplicate section": rawBundle(duplicate), "unsorted section": rawBundle(unsorted),
		"oversize claimed section": checksum(tooLong),
		"trailing body":            checksum(append(bytes.Clone(good[:len(good)-sha256.Size]), 0)),
		"trailing digest":          append(bytes.Clone(good), 0),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if got, _, err := Decode(data); !errors.Is(err, ErrBundle) || got != nil {
				t.Fatalf("accepted malformed frame: %v, %v", got, err)
			}
		})
	}
	bad := testSections()
	bad[2].Data = make([]byte, MaxManifestBytes+1)
	if _, _, err := Encode(bad); !errors.Is(err, ErrBundle) {
		t.Fatalf("oversize section accepted: %v", err)
	}
	bad = testSections()
	bad[1], bad[0] = bad[0], bad[1]
	if _, _, err := Encode(bad); !errors.Is(err, ErrBundle) {
		t.Fatalf("unsorted input accepted: %v", err)
	}
}

func TestFilePublicationStages(t *testing.T) {
	stages := []publicationStage{stageCreated, stageWritten, stageSynced, stageClosed, stageLinked, stageDirSynced}
	for _, failure := range stages {
		t.Run(string(failure), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "checkpoint.bundle")
			injected := errors.New("injected publication failure")
			_, err := writeFile(path, testSections(), func(stage publicationStage) error {
				if stage == failure {
					return injected
				}
				return nil
			})
			if !errors.Is(err, injected) {
				t.Fatalf("missed injection: %v", err)
			}
			if entries, err := os.ReadDir(dir); err != nil || len(entries) > 1 || (len(entries) == 1 && entries[0].Name() != "checkpoint.bundle") {
				t.Fatalf("left temporary/partial entries: %v, %v", entries, err)
			}
			got, _, err := ReadFile(path)
			if failure == stageLinked || failure == stageDirSynced {
				if err != nil || !reflect.DeepEqual(got, testSections()) {
					t.Fatalf("published incomplete final artifact: %v, %v", got, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("published before link: %v", err)
			}
		})
	}
}

func TestFileCleanupDirectorySync(t *testing.T) {
	for _, tc := range []struct {
		name             string
		failAfterDirSync bool
	}{
		{name: "error after first directory sync", failAfterDirSync: true},
		{name: "second directory sync fails"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "checkpoint.bundle")
			cleanupFailure := errors.New("injected cleanup directory sync failure")
			stageFailure := errors.New("injected post-publication failure")
			syncCalls := 0
			digest, err := writeFileWithSync(path, testSections(), func(stage publicationStage) error {
				if tc.failAfterDirSync && stage == stageDirSynced {
					return stageFailure
				}
				return nil
			}, func(dir string) error {
				syncCalls++
				if syncCalls == 2 {
					entries, readErr := os.ReadDir(dir)
					if readErr != nil || len(entries) != 1 || entries[0].Name() != "checkpoint.bundle" {
						t.Fatalf("cleanup sync ran before temp unlink: %v, %v", entries, readErr)
					}
					return cleanupFailure
				}
				return syncDirectory(dir)
			})
			if syncCalls != 2 || digest != [32]byte{} || !errors.Is(err, cleanupFailure) || (tc.failAfterDirSync && !errors.Is(err, stageFailure)) {
				t.Fatalf("cleanup sync not attempted/reported: calls=%d, error=%v", syncCalls, err)
			}
			got, _, readErr := ReadFile(path)
			if readErr != nil || !reflect.DeepEqual(got, testSections()) {
				t.Fatalf("final must remain complete: %v, %v", got, readErr)
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 1 || entries[0].Name() != "checkpoint.bundle" {
				t.Fatalf("temp not removed locally: %v, %v", entries, readErr)
			}
		})
	}
}

func TestFilePreexistingFinalPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.bundle")
	original := []byte("already owned by another writer")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteFile(path, testSections()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing final was not protected: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("preexisting content changed: %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file remained: %v, %v", entries, err)
	}
}

func TestFileImmutableReadAndPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "checkpoint.bundle")
	sections := testSections()
	digest, err := WriteFile(path, sections)
	if err != nil {
		t.Fatal(err)
	}
	sections[0].Data[0] ^= 1 // file owns the encoded bytes
	got, verified, err := ReadFile(path)
	if err != nil || verified != digest || !reflect.DeepEqual(got, testSections()) {
		t.Fatalf("read %v, %x, %v", got, verified, err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteFile(path, testSections()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second publication did not reject existing path: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("existing final changed: %v", err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Fatalf("temporary file remained: %v, %v", entries, err)
	}
	for _, bad := range []string{"", dir, dir + "/../escape", filepath.Join(dir, ".checkpoint-orphan")} {
		if _, err := WriteFile(bad, testSections()); !errors.Is(err, ErrPath) {
			t.Fatalf("write unsafe path %q: %v", bad, err)
		}
		if _, _, err := ReadFile(bad); !errors.Is(err, ErrPath) {
			t.Fatalf("read unsafe path %q: %v", bad, err)
		}
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFile(link); !errors.Is(err, ErrPath) {
		t.Fatalf("symlink final read: %v", err)
	}
	if _, err := WriteFile(link, testSections()); !errors.Is(err, os.ErrExist) {
		t.Fatalf("symlink final overwritten: %v", err)
	}
	if _, err := WriteFile(filepath.Join(link, "nested"), testSections()); !errors.Is(err, ErrPath) {
		t.Fatalf("symlink parent write: %v", err)
	}
	if _, _, err := ReadFile(filepath.Join(link, "nested")); !errors.Is(err, ErrPath) {
		t.Fatalf("symlink parent read: %v", err)
	}
	directoryLink := filepath.Join(dir, "directory-link")
	if err := os.Symlink(dir, directoryLink); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteFile(filepath.Join(directoryLink, "nested"), testSections()); !errors.Is(err, ErrPath) {
		t.Fatalf("symlink directory parent write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".checkpoint-unpublished"), before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFile(filepath.Join(dir, ".checkpoint-unpublished")); !errors.Is(err, ErrPath) {
		t.Fatalf("read unpublished temp: %v", err)
	}
	corrupt := bytes.Clone(before)
	corrupt[len(corrupt)/2] ^= 1
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFile(path); !errors.Is(err, ErrBundle) {
		t.Fatalf("corruption accepted: %v", err)
	}
	if err := os.WriteFile(path, before[:len(before)-1], 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFile(path); !errors.Is(err, ErrBundle) {
		t.Fatalf("truncation accepted: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxBundleBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadFile(path); !errors.Is(err, ErrBundle) {
		t.Fatalf("oversize file accepted: %v", err)
	}
}
