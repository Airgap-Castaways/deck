# Bundle Layout

`deck prepare` writes a self-contained workspace under the current directory because disconnected work gets harder when dependencies stay implicit.

The bundle is part of the product model, not an afterthought.

## Typical bundle contents

- `workflows/`: the workflow files copied into the bundle
- `outputs/packages/`: operating system or Kubernetes packages fetched during prepare
- `outputs/images/`: container image archives fetched during prepare
- `outputs/files/`: supporting files copied or downloaded during prepare
- `deck`: the current deck binary copied to the workspace root
- `.deck/manifest.json`: integrity manifest used by `bundle verify`
- `deck`: the `deck` binary placed in the bundle root
- `files/deck`: an additional bundled copy of the binary
- `.deck/manifest.json`: checksum metadata for bundled artifacts

## Why the bundle matters

- it keeps offline handoff explicit
- it reduces hidden runtime dependencies
- it makes the procedure easier to inspect before transport
- it supports the simple local execution model

## Core rule

If the site needs it to run the workflow, the safest default is to include it in the bundle rather than assume it already exists.
