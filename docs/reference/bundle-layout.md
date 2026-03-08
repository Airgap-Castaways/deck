# Bundle Layout

`deck pack` is built around a hermetic bundle contract. The offline site should receive everything needed to execute the workflow locally.

## Typical bundle contents

- `workflows/`: the workflow files copied into the bundle
- `packages/`: operating system or Kubernetes packages fetched during pack
- `images/`: container image archives fetched during pack
- `files/`: supporting files copied or downloaded during pack
- `deck`: the `deck` binary placed in the bundle root
- `files/deck`: an additional bundled copy of the binary
- `.deck/manifest.json`: checksum metadata for bundled artifacts

## Why the bundle matters

- It reduces hidden runtime dependencies inside the air gap.
- It keeps the operator handoff concrete: a bundle can be inspected, transferred, verified, and executed.
- It aligns with the project goal of being hermetic and self-contained.

## Core rule

If an offline site needs it to execute the workflow, the safest default is to make it part of the bundle rather than assuming it already exists.
