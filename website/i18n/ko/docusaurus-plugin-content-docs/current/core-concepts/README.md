---
source: docs/core-concepts/README.md
source_hash: 2d04b1fa8d3d92cdf11893ed99f345c5cd15bb41
---
# 개념

`deck`은 에어갭(망분리) 및 운영 제약이 있는 환경을 위한 로컬 우선(local-first) 워크플로 도구입니다. 이 섹션에서는 도구의 멘탈 모델을 설명합니다, 왜 존재하는지, 라이프사이클이 어떻게 작동하는지, 각 구성 요소가 어떻게 맞물리는지, 그래서 명령어가 *무엇*을 하는지뿐 아니라 *왜* 그렇게 설계되었는지까지 이해할 수 있도록 합니다.

## 이 섹션의 내용

- [왜 deck인가?](why-deck.md): deck이 해결하는 운영상의 문제와 이를 형성하는 핵심 원칙.
- [deck 라이프사이클](lifecycle.md): prepare → bundle → apply 모델: 각 단계가 무엇을 하는지, 번들 안에 무엇이 들어 있는지, 워크스페이스와 번들이 어떻게 다른지.
- [아키텍처](architecture.md): 시스템 경계, 장애 도메인(failure-domain) 모델, 그리고 deck의 설계가 운영자 수준에서 안전성과 예측 가능성을 어떻게 강제하는지.
