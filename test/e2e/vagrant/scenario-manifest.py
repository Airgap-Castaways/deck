#!/usr/bin/env python3

import json
import sys
from pathlib import Path


def scenario_basename(value: str) -> str:
    return value[4:] if value.startswith("k8s-") else value


def manifest_path(root: Path, scenario: str) -> Path:
    return root / "test" / "e2e" / "scenarios" / f"{scenario_basename(scenario)}.json"


def load_manifest(root: Path, scenario: str) -> dict:
    path = manifest_path(root, scenario)
    if not path.is_file():
        raise SystemExit(f"missing scenario manifest: {path}")
    return json.loads(path.read_text(encoding="utf-8"))


def print_metadata(manifest: dict) -> None:
    print(f"NODES={' '.join(manifest['nodes'])}")
    print(f"KUBERNETES_VERSION={manifest.get('kubernetesVersion', 'v1.30.1')}")
    print(f"UPGRADE_KUBERNETES_VERSION={manifest.get('upgradeKubernetesVersion', '')}")
    verify = manifest.get("verify", {})
    print(f"VERIFY_STAGE_DEFAULT={verify.get('defaultStage', '')}")


def print_actions(manifest: dict, phase: str, stage: str) -> None:
    if phase == "apply":
        actions = manifest.get("apply", [])
    elif phase == "verify":
        verify = manifest.get("verify", {})
        selected = stage or verify.get("defaultStage", "")
        actions = verify.get("stages", {}).get(selected, [])
    else:
        raise SystemExit(f"unsupported phase: {phase}")
    for action in actions:
        row = {
            "id": action["id"],
            "role": action["role"],
            "workflow": action["workflow"],
        }
        print(json.dumps(row, separators=(",", ":")))


def print_result(manifest: dict) -> None:
    print(json.dumps({
        "requiredArtifacts": manifest.get("requiredArtifacts", []),
        "resultEvidenceFiles": manifest.get("resultEvidenceFiles", {}),
    }, separators=(",", ":")))


def main() -> None:
    if len(sys.argv) < 4:
        raise SystemExit("usage: scenario-manifest.py <root> <scenario> <metadata|actions|result> [args]")
    root = Path(sys.argv[1]).resolve()
    scenario = sys.argv[2]
    command = sys.argv[3]
    manifest = load_manifest(root, scenario)
    if command == "metadata":
        print_metadata(manifest)
        return
    if command == "actions":
        phase = sys.argv[4] if len(sys.argv) > 4 else "apply"
        stage = sys.argv[5] if len(sys.argv) > 5 else ""
        print_actions(manifest, phase, stage)
        return
    if command == "result":
        print_result(manifest)
        return
    raise SystemExit(f"unsupported command: {command}")


if __name__ == "__main__":
    main()
