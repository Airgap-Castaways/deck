---
source: docs/cli/deck_ask_plan.md
source_hash: 5dc54fbc20c1a9e961505f19b294e2790704dd5d
---
## deck ask plan

워크플로 파일을 작성하지 않고 ask 플랜 아티팩트 생성

### Synopsis

워크플로 파일을 작성하지 않고 .deck/plan 아래에 재사용 가능한 플래닝 아티팩트를 생성합니다. 이 모드는 초안 작성/다듬기 스타일의 작성 요청을 위한 것입니다.

```
deck ask plan [request] [flags]
```

### Examples

```
  deck ask plan "create an air-gapped rhel9 single-node kubeadm workflow"
  deck ask plan --plan-name kubeadm-ha "create a 3-node kubeadm workflow"
```

### Options

```
      --answer stringArray   저장된 플랜 아티팩트에서 재개할 때 플랜 명확화 답변을 key=value 형식으로 적용
      --endpoint string      이번 실행에 한해 설정된 ask 프로바이더 엔드포인트를 재정의
      --from string          텍스트 또는 마크다운 파일에서 추가 요청 세부 정보를 로드
  -h, --help                 help for plan
      --model string         이번 실행에 한해 설정된 ask 모델을 재정의
      --plan-dir string      ask 플랜 아티팩트 디렉터리 (default ".deck/plan")
      --plan-name string     선택적 플랜 아티팩트 이름
      --provider string      이번 실행에 한해 설정된 ask 프로바이더를 재정의
```

### Options inherited from parent commands

```
      --log-format string   진단 로그 형식 (text|json) (default "text")
      --v int               진단 상세 수준 (0-3; 높을수록 더 상세함)
```

### SEE ALSO

* [deck ask](deck_ask.md)	 - (실험적) 워크플로 작성 및 검토를 위한 AI 헬퍼
