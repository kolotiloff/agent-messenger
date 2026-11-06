# Agent Messenger design foundation

On November 5, 2026 the user picked the style of the mockup shown in chat: calm surfaces, rounded corners, colors, spacing and element layout. We keep it as **design v1** and use it to implement the first window. The visual direction should not change without a separate discussion.

- [Openable mockup](v1/preview.html) — download the file and open it in a modern browser. GitHub shows the HTML source, not the app itself.
- [Source fragment](v1/concept.fragment.html) — the approved mockup; on November 6, at the owner's request, details of their work project were removed without changing the style.
- [Styles](v1/styles.css) — CSS extracted unchanged from the source fragment.
- [Design parameters](v1/tokens.json) — color names and sizes suitable for porting to whatever window technology is chosen.
- [Checksums](v1/manifest.json) — verification of the source fragment and CSS.

## What we keep

System font on macOS/Windows/Linux, 13 px body size, 18 px heading, weights 400/500. Window and card corner radius 12 px, buttons 8 px, fields 7 px. Side navigation 204 px, main area with symmetric 16 px padding, 12 px gaps between cards. The main conversation is the central part of the window. Section order in the product, agreed on November 6: **Переписка → Агенты → Сервер** ("Conversation → Agents → Server"). The conversation is open at launch. The saved original v1 mockup remains the archived style reference; the current order is implemented in the product window. Narrow windows reflow rather than shrinking the whole interface.

Colors are defined as `light-dark(...)` pairs: background, panels, text, dividers, selected items, a calm green status and an amber attention color. The light and dark themes are parts of one system. The archived v1 values live in the CSS and tokens.json; refinements based on the owner's new samples are recorded below and in the product styles.

Version v1 is a saved reference. When porting, we use these parameters in the product components. Accessibility, text scaling and platform requirements may be improved while keeping the overall look. Significant visual changes go into the next version with a description of the differences.

## Mockup boundaries

All conversations, machine statuses and actions are demo data. Approval of the design does not mean acceptance of the network, authorization, coordinator or the real window. The mockup does not call a CLI and does not connect to Messenger. Demo conversation is not saved as a real conversation.

preview.html contains a standalone shell for viewing and configuring the theme; helper scripts and icons load from pinned CDN addresses. It is not an offline distribution of the app. The source CSS and design parameters need no external network. The fragment's host integrations are limited to saving the selected view; in a regular browser the shell replaces them. Do not enter real addresses, passwords or keys into the mockup.

Full product requirements: [APP.md](../docs/APP.md); plan: [RELEASE_PLAN.md](../docs/RELEASE_PLAN.md).

## Window refinements of November 6

At the owner's request, the native window does not reproduce the browser mockup's outer frame and second header. The bottom bar was removed; the update notice sits at the bottom of the left panel. The message history scrolls, the input field is pinned, compact and has no prominent border. The agents section has three main provider cards; sessions open in a separate window with its own scrolling. CLI statuses are in the card, with no separate list below. These changes refine how the saved style is applied in the product.

Light/dark themes and «Как в системе» ("Match system") are selected at the bottom of the left panel. The neutral white and graphite surfaces were refined from the owner's screenshots of November 6, 09:26–09:27; this is Messenger's own palette, not extracted Claude styles or fonts. The modes were checked against the [official Claude help article](https://support.claude.com/en/articles/8887527-customizing-your-appearance-settings). The native app's choice is stored in user settings and does not depend on the local window's address.

The current product tokens are stored in `internal/localui/assets/app.css`: background #ffffff / #191919, sidebar #fafafa / #202020, text #303030 / #dddddd. Menu #ffffff / #303030, highlight #eeeeee / #4b4b4b, a blue checkmark on the selected item. Dropdowns open over panels, have their own scrolling and support arrows, Home/End, Enter and Escape. The input field is a white or graphite surface with a constant thin border: on focus only the text cursor appears. A small send icon sits on the right inside the shared field; there is no separate large button.

Vector provider icons were added to cards, session selection and the conversation. The sender is determined by the session's registered alias, not by the message text; an unknown sender is never given someone else's logo. The name and alias are kept side by side. Icons are embedded locally; viewing makes no requests to provider sites.

The repeated active-tab heading was removed. In the native app, refreshing data is in the menu «Вид → Обновить сообщения и статусы» ("View → Refresh messages and statuses"); only the queue toggle remains next to the tabs. Pause and resume are in the right-hand «Очередь сообщений» ("Message queue") panel next to the conversation. The technical phrase about model usage and the duplicate status at the bottom were removed; connectivity is shown by the computer indicator.

Action buttons are visible without hover: a constant fill #efefef / #404040, on hover and keyboard focus #e3e3e3 / #4b4b4b, no outline. Inactive actions have a muted fill and text. Tabs, computer selection and inline icons keep their own calm style.

Updates are detected automatically at launch, every 5 minutes and when returning to the window (with a shared interval of at least 5 minutes). «Доступно обновление» ("Update available") appears at the bottom left only when a version is available; the number is shown in the tooltip. Manual checking remains in the app menu. A network error is indicated by a «Повторить проверку» ("Retry check") button.

Implemented after preview.13: outgoing messages from the local window on the right in a soft block, agent replies on the left; the sender's name and icon are kept. For network mode, direction must be determined by the sender's confirmed association with the device, not by guessing from the alias. Until there is a network identifier, an arbitrary alias must not be presented as «этот компьютер» ("this computer"). This change arrived after the preview.13 build and is not included in it.

For outgoing messages: a soft rounded block with an even background, comfortable inner padding and no border/shadow. The reference is the owner's screenshot of a Claude conversation from 10:08. Incoming messages are on the left without an extra frame. Both themes use shared tokens.

## Unified product palette

After reviewing all components, the product styles were consolidated into `internal/localui/assets/app.css`; the frozen `design-v1.css` is no longer loaded by the window. All surfaces, text, buttons, fields, menus, states and messages take their colors from a single token set.

| Element | Light | Dark |
|---|---|---|
| Main background | #ffffff | #191919 |
| Sidebar | #fafafa | #202020 |
| Button / hover | #efefef / #e3e3e3 | #404040 / #4b4b4b |
| Select field / hover | #f3f3f3 / #eaeaea | #303030 / #393939 |
| Inactive action | #f5f5f5 | #282828 |
| Outgoing message | #f3f3f3 | #252525 |
| Primary / secondary text | #303030 / #6b6b6b | #dddddd / #a0a0a0 |

Buttons and the input field have no focus outlines. Keyboard focus on buttons is indicated by background. Select fields have a constant surface and a thin neutral border; they are visible without hover. An empty session group disables its button, and it stays disabled after statuses refresh. The bottom block is compact; the current version is taken from the installed bundle, separately from the available update.

The message queue is shown to the right of the conversation by default; an icon at the top hides and restores the panel. There is no separate queue tab. Currently the panel shows the number of pending messages and the sessions needing attention, based on client data. The queue panel, message history and computer list scroll independently; the theme, current version and update are pinned at the bottom left.


## Unified spacing grid

The product uses 4/8/12/16/24 px steps from the `--am-space-*` CSS variables. Blocks of the same type use the same values in both themes. The main panel padding is 16 px on each side; a nested component does not add another outer margin on top of it. In a narrow browser window up to 620 px, padding shrinks symmetrically to 12 px.

The session switcher, history and input field are aligned to a shared right content edge. The scrollbar sits at the panel edge, outside the text padding. Collapsing the queue does not change the left/right padding. The input field is pinned independently of the history.

Cards and the sessions window: 16 px inner padding, 12 px gaps between groups. Select menus: equal 8 px padding. Sidebar bottom block: the update button spans the full width, followed by the theme and installed version with an 8 px gap; the version number has no extra margin. The computer list and the bottom block share edges.

When changing layout, check both themes, queue open/closed, all tabs, menus and the sessions window, long names, multi-line input, normal and minimum window size. Check not just the screenshot but the padding geometry and the absence of whole-page scrolling.

WebKit/Chromium scrollbars have a constant 6 px width; hover changes only the color. The scrollbar's reserved space is accounted for in the content padding, so its appearance does not add a second right margin.
# Scrollbar calculation refinement

When computing symmetric padding, only the actual width of the occupied scrollbar gutter (`offsetWidth - clientWidth - borders`) is subtracted. With an overlay scrollbar it is zero. The width of the drawn thumb by itself does not prove that space is reserved.
