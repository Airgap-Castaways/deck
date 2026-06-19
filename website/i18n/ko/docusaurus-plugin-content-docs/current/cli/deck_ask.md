---
source: docs/cli/deck_ask.md
source_hash: f348c7b739825fddbec8cd6d2167e044b5935ec7
---
## deck ask

(실험적 기능) 워크플로 작성 및 검토를 위한 AI 헬퍼

```
deck ask [request] [flags]
```

### Examples

```
  deck ask "explain what workflows/scenarios/apply.yaml does"
  deck ask --create "create an air-gapped rhel9 single-node kubeadm workflow"
  deck ask --edit "refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml"
  deck ask plan "create an air-gapped rhel9 single-node kubeadm workflow"
```

### Options

```
      --answer stringArray   plan 아티팩트에서 재개할 때 plan 설명 답변을 key=value 형식으로 적용
      --create               요청을 새 워크플로 작성으로 처리
      --edit                 요청을 워크플로 개선으로 처리
      --endpoint string      이번 실행에 한해 구성된 ask 프로바이더 엔드포인트를 재정의
      --from string          텍스트 또는 마크다운 파일에서 추가 요청 세부 정보를 로드
  -h, --help                 help for ask
      --max-iterations int   draft/refine 경로의 최대 복구 시도 횟수 (0이면 경로 기본값 사용)
      --model string         이번 실행에 한해 구성된 ask 모델을 재정의
      --plan-dir string      ask plan 아티팩트 디렉터리 (default ".deck/plan")
      --plan-name string     ask plan에서 사용하는 선택적 plan 아티팩트 이름
      --provider string      이번 실행에 한해 구성된 ask 프로바이더를 재정의
      --review               파일을 작성하지 않고 현재 워크스페이스를 검토
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck
* [deck ask config](deck_ask_config.md)	 - 전역 ask 구성 기본값 및 API 자격 증명 관리
* [deck ask login](deck_ask_login.md)	 - OpenAI Codex OAuth로 ask 인증
* [deck ask logout](deck_ask_logout.md)	 - ask의 저장된 OAuth 세션 삭제
* [deck ask plan](deck_ask_plan.md)	 - 워크플로 파일을 작성하지 않고 ask plan 아티팩트 생성
* [deck ask status](deck_ask_status.md)	 - 저장된 ask OAuth 세션 상태 표시
