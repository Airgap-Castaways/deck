---
source: docs/cli/deck_apply.md
source_hash: 40f741622fe7a9f5d0118114283ff70874b0f939
---
 ## deck apply

bundle의 apply 파일을 실행합니다.

```
deck apply [workflow] [bundle] [flags]
```

### Options

```
      --dry-run              스텝을 실제로 실행하지 않고 apply 계획만 출력합니다
      --fresh                실행 전 저장된 apply 상태를 초기화합니다
  -h, --help                 apply에 대한 도움말을 표시합니다
      --non-interactive      프롬프트 없이 운영자 상호작용 스텝을 실패 처리하거나 기본값을 사용합니다
      --phase string         실행할 단계 이름(기본값: 모든 단계)
      --root string          workflows/ 디렉터리가 포함된 로컬 워크플로 루트
      --scenario string      실행할 시나리오 이름
      --server string        원격 워크플로 서버 URL
      --source string        시나리오 소스(local|server)(기본값: "local")
      --state-dir string     apply 상태 파일을 저장할 디렉터리(로컬 .deck/state/apply 또는 원격 XDG 상태를 재정의합니다)
      --var stringToString   변수를 덮어씁니다(key=value 형식). 여러 번 지정할 수 있습니다
  -f, --vars-file strings    지정한 워크플로 루트(workflows/) 기준 오버레이 vars 파일 경로. 여러 번 지정할 수 있습니다
      --workflow string      워크플로 파일의 경로 또는 URL
```

### 상위 명령어에서 상속된 옵션

```
      --log-format string   진단 로그 형식(text|json)(기본값: "text")
      --v int               진단 상세 수준(0-3). 값이 클수록 더 상세합니다
```

### 참고

* [deck](deck.md)	 - deck
