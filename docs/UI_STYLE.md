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

Compact boxes and rows (list rows, menu options, hints, code blocks) use `--am-pad-compact` (8 12). Panes are inset by `--am-pane-inset` (16, or 12 in a narrow window).

Buttons that belong together go in one container: `.am-actions` (a row) or `.am-actions-stack` (a column). Do not space them with margins.

## Radii

| Token | Size | Where |
|---|---|---|
| `--am-radius-xs` | 4 | tags inside text: inline code, role labels |
| `--am-radius-s` | 8 | buttons, fields, list rows, menu options, small boxes |
| `--am-radius-m` | 12 | cards, panels, notices, menus, error chips, the window |
| `--am-radius-l` | 16 | dialogs, the composer, message bubbles |

## Sizes

- Controls: `--am-control` 36 (buttons, fields, select fields, send and stop); `--am-control-compact` 32 (tabs, square icon buttons, the task filter); `--am-control-chip` 24 (buttons inside an error chip).
- Text: 11 labels and meta, 12 secondary text, 13 body, 15 titles (`h3`), 18 page titles (`h2`).
- Colors and shadows are tokens too (`--am-shadow-s` for chips and the composer, `--am-shadow-l` for menus and dialogs).

## Components

- `.am-card` is the one card look (agent cards, server sections); `.am-card-head` holds a title and a `.am-badge`.
- `.am-field` is a label with a field under it.
- Notes about a screen (what works, what does not yet) go in a side column, not between the cards: `.am-server-layout` with `.am-server-aside`. In a window narrower than 1000 px the column moves above the cards.
- `.am-icon-btn` is a square compact button; `.am-primary` the main action; `.am-danger` a destructive action.
- Status texts: `.am-status-good/-bad/-run/-wait`; dots: `.am-dot-run/-wait/-idle/-offline`.

## Checking a change

Take screenshots of the tasks, agents and server tabs, a dialog and a menu in both themes at 1100 and 600 px, and measure button and field heights: they must be 36, 32 or 24.
