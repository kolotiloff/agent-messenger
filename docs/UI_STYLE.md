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

Compact boxes and rows (list rows, hints, code blocks) use `--am-pad-compact` (8 12). Dropdown menus follow the Claude app: a 4 px inset, 32 px options (`--am-control`) with 4 12 padding, only the hovered or keyboard-focused option gets a background, and the chosen option is marked by a thin accent check on the right. Panes are inset by `--am-pane-inset` (16, or 12 in a narrow window).

Buttons that belong together go in one container: `.am-actions` (a row) or `.am-actions-stack` (a column). Do not space them with margins.

## Radii

| Token | Size | Where |
|---|---|---|
| `--am-radius-xs` | 4 | tags inside text: inline code, role labels |
| `--am-radius-s` | 8 | buttons, fields, list rows, menu options, small boxes |
| `--am-radius-m` | 12 | cards, panels, notices, menus, error chips, the window |
| `--am-radius-l` | 16 | dialogs, the composer, message bubbles |

The composer keeps its surface radius (16 px) at every height. The inset icon controls use the next smaller radius (12 px). Ordinary buttons and selector triggers use the compact control height (28 px) and neutral gray fill in the current window layout.

The conversation and composer share a centered column up to `--am-chat-max` (760 px), with a minimum chat gutter of 24 px on each side. The scrollbar stays at the edge of the chat pane.

## Sizes

- Controls: `--am-control` 32 (buttons, fields, select fields, send and stop), the size of buttons in the Claude app; `--am-control-compact` 28 (tabs, square icon buttons, the task filter); `--am-control-chip` 24 (buttons inside an error chip).
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

Dialogs that ask for a decision (`.am-alert`) use 16 px padding, the standard 16 px section heading, muted text, the leaving or destructive choice on the left and the main action on the right. A destructive button (`.am-danger`) has red text on a soft red fill.

Text in a control is centred by eye, not by the line box: system fonts leave more room below the text than above it, so buttons, tabs, select fields and inputs get `--am-text-nudge` (4) as their top padding and none at the bottom. Measured: 11 px above the capitals and 11 below in a 32 px button.
