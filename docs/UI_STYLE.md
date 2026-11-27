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
| Composer shell | 4 px | 4 px |
| Computer headings, active task rows, statistics task links and efficiency model rows | 4 px | 12 px, with a 36 px leading inset on active tasks to align the titles |
| Archive rows and compact notices | 8 px | 12 px |
| Report cards, expanded efficiency details and outgoing message bubbles | 12 px | 12 px |
| Popover menus | 8 px | 8 px |
| Settings cards | 16 px | 16 px |

Expanded efficiency details start 16 px inside the parent row and share its trailing edge. The 12 px inner inset aligns the first metric label with the model name. The detail table adds no extra padding at its outer edges.

Settings section headings sit above the first card with the section gap. Computer and statistics rows use the quiet hover fill. Dropdown options have 4 px vertical and 12 px horizontal padding. Only the hovered or keyboard-focused option gets a background, and a check marks the selected value. Panes use `--am-pane-inset` (16 px, or 12 px in a narrow window). The Projects divider aligns with the full-width task toolbar. The Archive header starts below the toolbar. The Clear Archive button uses the compact filled style and starts at the same inset as the archive rows. The footer aligns the visible icon and version text edges by applying the icon's internal inset to the version text.

Buttons that belong together go in one container: `.am-actions` (a row) or `.am-actions-stack` (a column). Do not space them with margins.

## Radii

| Token | Size | Where |
|---|---|---|
| `--am-radius-xs` | 4 | tags inside text: inline code, role labels |
| `--am-radius-s` | 8 | buttons, fields, list rows, menu options, small boxes |
| `--am-radius-m` | 12 | cards, panels, notices, menus, error chips, the window |
| `--am-radius-l` | 16 | dialogs, the composer, message bubbles |

The composer keeps its surface radius (16 px) at every height. The inset icon controls use the next smaller radius (12 px). Ordinary buttons, one-line inputs and selector triggers share the compact control height (28 px) in the current window layout.

The conversation and composer share a centered column up to `--am-chat-max` (760 px), with a minimum chat gutter of 24 px on each side. The scrollbar stays at the edge of the chat pane.

## Sizes

- Controls: `--am-control` 32 (composer controls and drop-down options), `--am-control-compact` 28 (ordinary one-line fields, buttons, tabs and select triggers in the current layout), `--am-control-chip` 24 (buttons inside an error chip).
- Text: 12 labels and meta, 13 secondary text, 14 body, 16 section and dialog titles (`h3`), 18 page titles (`h2`). Computer names, task names in the list and statistics use 14 px, weight 500, with a 21 px line height. Sidebar agent rows and task card metadata use 12 px. Chat content and input use 14 px regular system text with a 21 px line height.
- Colors and shadows are tokens too (`--am-shadow-s` for chips and the composer, `--am-shadow-l` for menus and dialogs).

## Components

- `.am-card` is the one card look (agent cards, server sections); `.am-card-head` holds a title and a `.am-badge`.
- `.am-field` is a label with a field under it.
- Notes about a screen (what works, what does not yet) go in a side column, not between the cards: `.am-server-layout` with `.am-server-aside`. In a window narrower than 1000 px the column moves above the cards.
- `.am-icon-btn` is a square compact button; `.am-primary` the main action; `.am-danger` a destructive action.
- Status texts: `.am-status-good/-bad/-run/-wait`; dots: `.am-dot-run/-idle/-offline`.

## Checking a change

Review interface changes in a running local preview of the real window assets. Start `node scripts/preview-window.cjs` and open the printed loopback address in the in-app browser. Use fictional data to try the affected interactions and refresh the preview as the changes are made. Share the live preview for design feedback.

Check the tasks, agents and server tabs, a dialog and a menu in both themes at 1100 and 600 px. Measure button and field heights against the 32, 28 and 24 px control tokens. Capture screenshots when needed for a verification report.

Standard dialogs share the 44 px header with 4 px vertical and 16 px horizontal padding. Dialogs that ask for a decision (`.am-alert`) use 16 px padding and a 12 px gap between the heading, text and actions. They use the standard 16 px section heading, muted text, the leaving or destructive choice on the left and the main action on the right. A destructive button (`.am-danger`) has red text on a soft red fill.

Text in a control is centred by eye, not by the line box: system fonts leave more room below the text than above it, so buttons, tabs, select fields and inputs get `--am-text-nudge` (4) as their top padding and none at the bottom. Measured: 11 px above the capitals and 11 below in a 32 px button.
