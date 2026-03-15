# Bundle Layout

`deck prepare` writes a self-contained workspace under the current directory because disconnected work gets harder when dependencies stay implicit.

The bundle is part of the product model, not an afterthought.

## Canonical bundle inputs

`deck bundle build` archives the canonical workspace inputs below.

- `deck`: the current deck binary copied to the workspace root during `prepare`
- `workflows/`: scenario, component, and variable files used at the site
- `outputs/packages/`: operating system or Kubernetes packages fetched during prepare
- `outputs/images/`: container image archives fetched during prepare
- `outputs/files/`: supporting files copied or downloaded during prepare
- `.deck/manifest.json`: integrity manifest used by `bundle verify`

`bundle build` does not archive arbitrary extra root-level paths by default. If a workflow needs additional content at the site, that content should be modeled under `workflows/` or produced under `outputs/` so it becomes part of the canonical bundle.

## Why the bundle matters

- it keeps offline handoff explicit
- it reduces hidden runtime dependencies
- it makes the procedure easier to inspect before transport
- it supports the simple local execution model

## Core rule

If the site needs it to run the workflow, the safest default is to place it in the canonical bundle inputs rather than assume it already exists.
