# docs/examples Test Cases

`docs/examples`는 문서 예제이면서 동시에 Vagrant 회귀 테스트 입력으로 사용한다.

## 케이스 목록

- 케이스 인덱스: `docs/examples/cases.tsv`
- 컬럼
  - `file`: 예제 YAML 파일명
  - `mode`: `validate` 또는 `run-install`

## 실행 방법

로컬 libvirt Vagrant 실행:

```bash
test/vagrant/run-examples.sh
```

검증 스크립트:

```bash
test/vagrant/verify-examples.sh <artifact-dir>
```

## 아티팩트

- summary: `.ci/artifacts/examples-<ts>/examples-summary.tsv`
- case log: `.ci/artifacts/examples-<ts>/cases/*.log`
- marker: `.ci/artifacts/examples-<ts>/vagrant-smoke-marker.txt`

`run-install` 케이스는 VM 내부 marker(`/tmp/deck/examples/vagrant-smoke.txt`)를 생성해 실행 성공을 검증한다.
