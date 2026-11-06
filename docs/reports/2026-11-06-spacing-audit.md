# Interface spacing grid audit

November 6, 2026. Product HTML/CSS/JS, a demo server with no writes to the client and no models.

Checked: both themes, the conversation with the queue open/closed, the three provider cards, the server, a window of 14 sessions with its own scrolling, select menus. In 900×650 and 1280×800 windows the input padding was measured: exactly 16 px on the left and right relative to the conversation panel. The scrollbar coincides with its right edge, and when open, with the queue's border. The select field and the input share a right edge.

Growing the input to 140 px was checked: the last message stays above the fixed input block, and the history's bottom position is preserved. The empty session group's button is disabled. The bottom block shows demo installed/available versions: the full-width button, theme selection and version are separated by an 8 px gap. The demo update availability is not a real release.

In the minimum 780×570 window, content does not overflow, the input and bottom settings are accessible, and padding stays equal.

Go UI tests with race detection passed. This is a check of browser styling on Mac; it does not replace acceptance of native Windows/Linux packages.


After the scrollbar change, the actual 6 px width and 16 px padding at the conversation, cards and sessions window were measured. The hover rule changes only the color; the width is constant. For the minimum window, no horizontal overflow and an accessible input were confirmed separately.

The first CI run found a race in the test bench `TestAdapterLauncherResumesExactID`: the arguments file was read after the first printf fragment, before writing finished. Fixed with an explicit end-of-write marker for both files and waiting for the supervisor to finish on any test exit. This is a test change, not a weakening of the product resume checks.
