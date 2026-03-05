# Vagrant CI Scripts

이 디렉터리는 GitHub Actions의 Vagrant 기반 E2E 검증 스크립트와 VM 정의를 포함한다.

## 구성 파일

- `Vagrantfile`: control-plane/worker 2노드 VM 정의
- `run-smoke.sh`: main push 스모크 시나리오
- `run-nightly.sh`: nightly 클러스터 코어 시나리오
- `run-offline-single-node-real.sh`: single-node 오프라인(egress 차단) repo+registry 설치 시나리오
- `scripts/run-cluster-core-scenario.sh`: VM 내부 클러스터 코어 설치 시나리오
- `verify-cluster-core-artifacts.sh`: 클러스터 코어 아티팩트 검증

## 시나리오 요약

- control-plane
  - 로컬 번들(`.ci/cache/prepare/*/bundle`) 마운트 경로 사용
  - `InstallPackages(source=local-repo)` + `KubeadmInit(mode=real)` 수행
- worker
  - 로컬 번들(`.ci/cache/prepare/*/bundle`) 마운트 경로 사용
  - `InstallPackages(source=local-repo)` + `KubeadmJoin(mode=real)` 수행

## 아티팩트

- `.ci/artifacts/smoke-*/`
- `.ci/artifacts/nightly-*/`
- `.ci/artifacts/offline-single-node-*/`

각 아티팩트 디렉터리에는 상태 파일, manifest, vagrant status가 저장된다.

nightly 모드는 클러스터 노드 목록(`kubectl get nodes`)과 기본 아티팩트를 검증한다.
