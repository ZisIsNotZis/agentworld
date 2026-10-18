#!/usr/bin/env python3
"""Reduce a live evaluation artefact to the rows `vrd2.py --live` actually reads.

The run artefacts live under git-ignored `.tmp/`, so ticket 42's scored facts had no
durable home. This cuts each artefact down to `vrd2.py`'s own LIVE_ROW_KEYS per cell plus
the three document fields `--live` prints (contract, horizon, status) and the inventory
block it asserts, so the reduced file is an exact subset of the artefact the run produced
rather than a transcription of it.

    python3 .scratch/42-m2d-replication/extract-live-rows.py \
        .tmp/42-f12-s0-evaluation.json .scratch/42-m2d-replication/f12-s0-live-extract.json
"""

import importlib.util
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
SCORER = os.path.join(HERE, os.pardir, "38-m2c-preregistration", "vrd2.py")
DOC_KEYS = ("claim_contract", "horizon_hours", "status")

spec = importlib.util.spec_from_file_location("vrd2", SCORER)
vrd2 = importlib.util.module_from_spec(spec)
spec.loader.exec_module(vrd2)


def main(argv):
    if len(argv) != 3:
        sys.stderr.write(__doc__.splitlines()[0] + "\n")
        raise SystemExit(2)
    src, dst = argv[1], argv[2]
    with open(src, encoding="utf-8") as fh:
        doc = json.load(fh)
    rows = []
    for row in doc.get("cells") or []:
        missing = [k for k in vrd2.LIVE_ROW_KEYS if k not in row]
        if missing:
            raise SystemExit("%s: cell %s/%s lacks %s — absence is not zero"
                             % (src, row.get("arm"), row.get("replicate"), ", ".join(missing)))
        out = {k: row[k] for k in vrd2.LIVE_ROW_KEYS}
        rows.append(out)
    reduced = {k: doc.get(k) for k in DOC_KEYS}
    reduced["inventory"] = doc.get("inventory")
    reduced["cells"] = rows
    reduced["reduced_from"] = os.path.relpath(src)
    reduced["reduced_by"] = os.path.relpath(os.path.abspath(__file__))
    reduced["reduction"] = ("exactly vrd2.LIVE_ROW_KEYS per cell + the three document fields --live "
                            "prints + the inventory block it asserts; every other artefact field is "
                            "dropped, and nothing is edited")
    with open(dst, "w", encoding="utf-8") as fh:
        json.dump(reduced, fh, indent=1, sort_keys=True)
        fh.write("\n")
    print("wrote %s: %d cells, %d bytes" % (dst, len(rows), os.path.getsize(dst)))


if __name__ == "__main__":
    main(sys.argv)
