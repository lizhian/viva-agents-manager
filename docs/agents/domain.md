# 领域文档

本仓库的领域文档应如何在工程类 skill 探索代码库时被消费。

## 探索之前，先读这些

- 仓库根目录的 **`GLOSSARY.md`**，或
- 仓库根目录的 **`GLOSSARY-MAP.md`**（如果存在）：它指向每个上下文各自的 `GLOSSARY.md`。读取与当前主题相关的每一份。
- **`docs/adr/`**：读取涉及你即将改动区域的 ADR。在多上下文仓库中，还要检查 `src/<context>/docs/adr/` 里的上下文级决策。

如果这些文件不存在，**静默继续**。不要指出它们缺失，也不要在开头建议创建它们。`/domain-modeling` skill（经 `/grill-with-docs` 与 `/improve-codebase-architecture` 进入）会在术语或决策真正落地时才惰性创建它们。

## 文件结构

单上下文仓库（大多数仓库）：

```
/
├── GLOSSARY.md
├── docs/adr/
│   ├── 0001-event-sourced-orders.md
│   └── 0002-postgres-for-write-model.md
└── src/
```

多上下文仓库（根目录存在 `GLOSSARY-MAP.md`）：

```
/
├── GLOSSARY-MAP.md
├── docs/adr/                          ← 系统级决策
└── src/
    ├── ordering/
    │   ├── GLOSSARY.md
    │   └── docs/adr/                  ← 上下文级决策
    └── billing/
        ├── GLOSSARY.md
        └── docs/adr/
```

## 使用词表的词汇

当你的产出要命名某个领域概念时（issue 标题、重构提案、假设、测试名称），使用 `GLOSSARY.md` 中定义的术语。不要漂移到词表明确避免的同义词。

如果你需要的概念还不在词表里，这是一个信号：要么你在发明项目并不使用的语言（重新考虑），要么确实存在缺口（记下来，留给 `/domain-modeling`）。

## 标注 ADR 冲突

如果你的产出与既有 ADR 相矛盾，明确指出来，而不是悄悄覆盖：

> _与 ADR-0007（事件溯源的订单）相矛盾，但值得重新讨论，因为……_
