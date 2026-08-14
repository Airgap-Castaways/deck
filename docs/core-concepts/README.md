# Concepts

`deck` is a local-first workflow tool for air-gapped and operationally constrained environments. This section explains the mental model behind the tool: why it exists, how its lifecycle works, and how the pieces fit together. The goal is to help you understand not just what the commands do, but why they are designed the way they are.

## In this section

- [Why deck?](why-deck.md): the operational problem deck solves and the core principles that shape it.
- [The deck lifecycle](lifecycle.md): the prepare → bundle → apply model: what each stage does, what lives in a bundle, and how workspaces and bundles differ.
- [Architecture](architecture.md): system boundaries, the failure-domain model, and how deck's design enforces safety and predictability at the operator level.
