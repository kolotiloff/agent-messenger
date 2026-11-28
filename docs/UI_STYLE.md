# Window style rules

The window's stylesheet is `internal/localui/assets/app.css`. Its tokens are defined once at the top of `#am-concept`. New rules use the tokens, not raw sizes. The frozen `design/v1` concept is separate and is not edited.

## Spacing

Scale: 4, 8, 12, 16, 24 (`--am-space-1/2/3/4/6`). Pick the gap by meaning:

| Token | Size | Where |
|---|---|---|
| `--am-gap-control` | 8 | between controls of one group (buttons in a row or column, fields), between a label and its field |
| `--am-gap-content` | 12 | between blocks inside a card, panel or pane |
| `--am-gap-section` | 16 | between cards and sections; the padding of a card |
| `--am-gap-region` | 24 | between large regions (sidebar groups, message groups) |
| `--am-gap-line` | 2 | only between lines of text in one item |

| Surface | Vertical inset | Horizontal inset |
|---|---:|---:|
| Composer shell | 8 px | 8 px |
| Computer headings, active task rows, statistics task links and efficiency model rows | 4 px | 12 px, with a 36 px leading inset on active tasks to align the titles |
| Compact notices | 8 px | 12 px |
| Report cards, expanded efficiency details and outgoing message bubbles | 12 px | 12 px |
| Popover menus | 8 px | 8 px |
| Settings cards | 16 px | 16 px |

Expanded efficiency details start 16 px inside the parent row and share its trailing edge. The 12 px inner inset aligns the first metric label with the model name. The detail table adds no extra padding at its outer edges.

Window notices stay in the same upper corner when Settings opens. Leave the final period off short notice titles. Keep punctuation in explanatory text and keep the ellipsis on actions in progress.

The sidebar action and project filter, statistics periods and filter controls use the same 8 px control gap. Compact list rows keep their 4 px gap. The first computer begins 8 px below the Mac project filter. Settings section headings sit above the first card with the section gap. Computer, Archive and Statistics task rows share the list hover fill. Statistics tasks have the same 4 px gap as archived tasks. Dropdown options have 4 px vertical and 12 px horizontal padding. Only the hovered or keyboard-focused option gets a background, and a check marks the selected value. Panes use `--am-pane-inset` (16 px, or 12 px in a narrow window). The Projects divider aligns with the full-width task toolbar. The Archive header starts below the toolbar. The quiet Clear Archive icon sits beside the count and opens a confirmation dialog. The footer aligns the visible icon and version text edges by applying the icon's internal inset to the version text.

The reply footer aligns the cap-height center of its selected-font labels with the shield and context meter. The model uses the primary text color. The 16 px meter glyph sits in the standard 28 px compact button with the 8 px radius. Its measured portion uses the accent color.

Buttons that belong together go in one container: `.am-actions` (a row) or `.am-actions-stack` (a column). Do not space them with margins.

## Radii

| Token | Size | Where |
|---|---|---|
| `--am-radius-xs` | 4 | tags inside text: inline code, role labels |
| `--am-radius-s` | 8 | buttons, fields, list rows, menu options, small boxes |
| `--am-radius-m` | 12 | cards, panels, notices, menus, error chips, the window |
| `--am-radius-l` | 16 | dialogs, the composer, message bubbles |

In the wide task window, Projects and Archive use the section inset (`--am-gap-section`, 16 px). The Mac caption divider runs across both panels. The archive header starts 8 px below the toolbar, with a 4 px gap before its first row. Align the heading and the empty state with archive row text. Place the clear icon beside the archive count and the horizontal sliders for filtering at the trailing edge. Computer and folder headings share a row height and icon size. Computer, folder and task rows use a 4 px gap at every nesting level. Both filters use the shared select styles. Active and archived task rows use the same 4 px vertical and 12 px horizontal padding. The active tree adds a 24 px leading indent to align the title with its computer. Task rows use the soft surface on hover and the quieter task selection fill for the current conversation. Author labels sit one control gap (8 px) above their messages.

The composer keeps its surface radius (16 px) at every height. Use one control gap (8 px) for padding on all four sides and for spacing between controls. The inset icon controls use the next smaller radius (12 px). Ordinary buttons, one-line inputs and selector triggers share the compact control height (28 px) in the current window layout.

The conversation and composer share a centered column up to `--am-chat-max` (760 px), with a minimum chat gutter of 24 px on each side. The scrollbar stays at the edge of the chat pane and ends at the footer. In an existing conversation the access shield and model sit below the reply field.

Message actions and the timestamp share a visible row one control gap (8 px) below the message. The executor's timestamp starts at the text edge, followed by the actions. An outgoing row ends at the bubble edge, with the timestamp before the actions. The next message group starts one region gap (24 px) below the row.

## Sizes

- Controls: `--am-control` 32 (composer controls and drop-down options), `--am-control-compact` 28 (ordinary one-line fields, buttons, tabs and select triggers in the current layout), `--am-control-chip` 24 (buttons inside an error chip).
- Text: 12 labels and meta, 13 secondary text, 14 body, 16 section and dialog titles (`h3`), 18 page titles (`h2`). Use regular weight for interface labels, titles and author names, with grayscale font smoothing where supported. The task sidebar uses 14 px for Projects, computer names and task titles. Preserve emphasis authored in chat content. Sidebar agent rows and task card metadata use 12 px. Chat content and input use 14 px regular system text with a 21 px line height.
- Use `--am-list-row-hover` for hover and keyboard focus on sidebar computer headers, task rows, Archive rows and interactive Statistics rows.
- Colors and shadows are tokens too (`--am-shadow-s` for chips and the composer, `--am-shadow-l` for menus and dialogs).

## Components

- `.am-card` is the one card look (agent cards, server sections); `.am-card-head` holds a title and a `.am-badge`.
- In Voice, use the page title for the section name and standard setting-label typography inside each card.
- `.am-field` is a label with a field under it.
- Notes about a screen (what works, what does not yet) go in a side column, not between the cards: `.am-server-layout` with `.am-server-aside`. In a window narrower than 1000 px the column moves above the cards.
- `.am-chrome-icon` shares the 28 px square, 16 px glyph box, 8 px corner radius and hover fill for standalone main-window controls.
- Task provider marks use 12 px. The chat author, access and context icons, inline tool actions, computer and folder icons, and agent settings marks use 16 px. Provider logos in the Archive filter use the same 16 px box. Drop layout illustrations use 32 px. The Windows caption uses 14 px glyphs in system-style controls. The existing updater glyph retains its 18 px drawing box.
- The reply footer centers the model, access shield and context ring on the same line. The shield carries the current permission in its accessible label. The draft shield opens the permission menu. Attachment chips sit above the compose field, with the slash menu opening above the chips. In the draft model menu, scroll the option list within the menu insets and keep search above it.
- `.am-icon-btn` is a square compact button; `.am-primary` the main action; `.am-danger` a destructive action.
- Status texts: `.am-status-good/-bad/-run/-wait`; dots: `.am-dot-run/-idle/-offline`.

## Checking a change

Review interface changes in a running local preview of the real window assets. Start `node scripts/preview-window.cjs` and open the printed loopback address in the in-app browser. Use fictional data to try the affected interactions and refresh the preview as the changes are made. Share the live preview for design feedback.

Check the tasks, agents and server tabs, a dialog and a menu in both themes at 1100 and 600 px. Measure button and field heights against the 32, 28 and 24 px control tokens. Capture screenshots when needed for a verification report.

Standard dialogs share the 44 px header with 4 px vertical and 16 px horizontal padding. Dialogs that ask for a decision (`.am-alert`) use 16 px padding and a 12 px gap between the heading, text and actions. They use the standard 16 px section heading, muted text, the leaving or destructive choice on the left and the main action on the right. A destructive button (`.am-danger`) has red text on a soft red fill.

In a dialog or popover with an overflowing list, keep the title, search and filters in a fixed header. Scroll only the content below the header. Keep the scrollbar inside the content area, clear of the top edge and the rounded corners. The model picker and agent limits follow this pattern.

Informational popovers such as agent limits and the context window use `--am-space-3` (12 px) vertically and `--am-space-4` (16 px) horizontally. Compact menu item spacing belongs to action rows.

Text in a control is centred by eye, not by the line box: system fonts leave more room below the text than above it, so buttons, tabs, select fields and inputs get `--am-text-nudge` (4) as their top padding and none at the bottom. Measured: 11 px above the capitals and 11 below in a 32 px button.

The task sidebar uses a 4 px gap between New Task, Projects and the computer list. Projects starts on the icon column. A collapsed computer or project folder shows its current task total after its label, using the same muted numeral as the heading counts. Expanded groups omit the numeral. Only outermost groups have a new task shortcut. The plus uses the row padding at the trailing edge and has a transparent hit area and appears on row hover or keyboard focus. Show it by default when hover is unavailable. Each task highlight starts at its nesting level. Task and archive buttons share 4 px vertical and 12 px horizontal padding. Both filters use a 16 px horizontal sliders icon in a compact button. The layers button groups tasks by computer, by project, by computer then project, or by project then computer. Folders in both states and computers use the same 16 px box, drawing bounds and stroke weight. Both folder states share the tab. In the open state the rounded front lip projects beyond the rear wall. Online computers use a muted server glyph. A slash and an accessible offline label identify disconnected computers.

Mutually exclusive settings pages can use `.am-segmented-tabs` with equal-width segments and a single inset indicator on a soft track. Only the indicator moves, over 160 ms after user input. Reduced motion switches it immediately. Use tab and tabpanel semantics, arrow-key navigation and a visible keyboard focus ring. Navigating between tabs changes the view only.
