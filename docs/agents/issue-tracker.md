# Issue tracker：GitHub

本仓库的 issue 与 spec 以 GitHub issue 形式存放。所有操作使用 `gh` CLI。

## 约定

- **创建 issue**：`gh issue create --title "..." --body "..."`。多行正文使用 heredoc。
- **读取 issue**：`gh issue view <number> --json number,title,body,labels,comments`。
- **列出 issue**：`gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`，配合适当的 `--label` 与 `--state` 过滤。
- **把 issue 设为某个父 issue 的子 issue**：`gh issue create --parent <parent> ...`，或事后 `gh issue edit <parent> --add-sub-issue <child>`（`gh` 2.94+）。更早的 `gh`：`gh api --method POST repos/<owner>/<repo>/issues/<parent>/sub_issues -F sub_issue_id=<child-db-id>`（数据库 id，见下文 **阻塞关系**）。若 sub-issue 不可用，在子 issue 正文顶部写 `Part of #<parent>`。
- **阻塞关系**：使用 GitHub **原生 issue 依赖**，这是规范且 UI 可见的表示方式。用 `gh issue create --blocked-by <n>,<n> ...` 或 `gh issue edit <child> --add-blocked-by <blocker>` 添加边（`gh` 2.94+）。更早的 `gh`：`gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`，其中 `<blocker-db-id>` 是阻塞者的数字**数据库 id**（`gh api repos/<owner>/<repo>/issues/<n> --jq .id`，_不是_ `#number`，也不是 `node_id`）。依赖功能不可用时，退化为在子 issue 正文顶部写 `Blocked by: #<n>, #<n>` 一行。当所有阻塞者都已关闭时，该 ticket 即解除阻塞。
- **统计未关闭的阻塞者**：`gh api repos/<owner>/<repo>/issues/<n> --jq .issue_dependencies_summary.blocked_by`（仅统计未关闭的阻塞者；为 0 表示可以开始）。`gh issue view <n> --json blockedBy` 也会列出已关闭的阻塞者，需要过滤：`--jq '[.blockedBy.nodes[] | select(.state == "OPEN")] | length'`。
- **在 issue 下评论**：`gh issue comment <number> --body "..."`
- **添加 / 移除标签**：`gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **关闭**：先用 `gh issue comment <number> --body "..."` 发布说明，再 `gh issue close <number>`。对已关闭的 issue，`gh issue close --comment` 会丢弃评论但仍以 0 退出。

从 `git remote -v` 推断仓库；在 clone 内运行时 `gh` 会自动这样做。

只向用户自己的仓库提交 issue。除非用户拥有该仓库，否则永远不要提交到 `mattpocock/skills`（这些 skill 的来源仓库）。在 fork 中，`gh` 可能默认指向上游父仓库，因此每次会话第一次 `gh issue create` 之前，先用 `gh repo view --json nameWithOwner` 检查目标仓库；如果它不是用户的仓库，就与用户确认。

## PR 作为 triage 入口

**PR 作为请求入口：否。** _（若本仓库把外部 PR 当作功能请求，改为 `yes`；`/triage` 会读取这个标志。）_

设为 `yes` 时，PR 与 issue 走同一套标签和状态，使用对应的 `gh pr` 命令：

- **读取 PR**：`gh pr view <number>`，然后 `gh pr view <number> --comments`（非 TTY 环境下 `--comments` 只打印评论，不打印正文），diff 用 `gh pr diff <number>`。
- **列出待 triage 的外部 PR**：`gh api --paginate 'repos/{owner}/{repo}/pulls?state=open' --jq '.[] | select(.author_association | IN("OWNER","MEMBER","COLLABORATOR") | not) | {number, title, author: .user.login, author_association, labels: [.labels[].name]}'`。
- **评论 / 打标签 / 关闭**：`gh pr comment`、`gh pr edit --add-label`/`--remove-label`、`gh pr close`。

GitHub 的 issue 与 PR 共用一个编号空间，所以裸写的 `#42` 可能是两者之一：先用 `gh pr view 42` 解析，再回退到 `gh issue view 42`。

## 当某个 skill 说「发布到 issue tracker」

创建一个 GitHub issue。

## 当某个 skill 说「获取相关 ticket」

按上文 **读取 issue** 的方式读取。

## Wayfinding 操作

供 `/wayfinder` 使用。**map** 是一个 issue，**child** issue 是它的 ticket。

- **Map**：单个打上 `wayfinder:map` 标签的 issue，承载 Notes / Decisions-so-far / Fog 正文。`gh issue create --label wayfinder:map`。
- **Child ticket**：作为 GitHub sub-issue 链接到 map 的 issue（见 **把 issue 设为某个父 issue 的子 issue**）。sub-issue 未启用时，把 child 加入 map 正文的任务列表，并在 child 正文顶部写 `Part of #<map>`。标签：`wayfinder:<type>`（`research`/`prototype`/`grilling`/`task`）。被认领后，该 ticket 指派给负责的开发者。
- **阻塞**：见 **约定** 下的 **阻塞关系**。
- **前沿查询（Frontier query）**：列出 map 未关闭的 children（`gh issue list --state open`，限定为 map 的 sub-issue / 任务列表），剔除存在未关闭阻塞者的（见 **统计未关闭的阻塞者**，或 `Blocked by` 行中未关闭的 issue）以及已有指派人的；按 map 顺序取第一个。
- **认领**：`gh issue edit <n> --add-assignee @me`，这是本会话的第一次写操作。
- **解决**：`gh issue comment <n> --body "<answer>"`，然后 `gh issue close <n>`，再把上下文指针（gist + 链接）追加到 map 的 Decisions-so-far。
