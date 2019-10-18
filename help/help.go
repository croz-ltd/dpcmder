package help

import (
	"github.com/croz-ltd/dpcmder/extprogs"
)

var Help = `dpcmder Help

ArrowUp / i         - move one item up
ArrowDown / k       - move one item down
Shift+ArrowUp / I   - select current item and move one item up (can use Alt instead of Shift)
Shift+ArrowDown / K - select current item and move one item down (can use Alt instead of Shift)
PgUp / u            - move page of items up
PgDown / o          - move page of items down
Shift+PgUp / U      - select current item and move page of items up
Shift+PgDown / O    - select current item and move page of items down
Home / a            - move to first item
End / z             - move to last item
Shift+Home / A      - move to first item and select all items from current one to the first one
Shift+End / Z       - move to last item and select all items from current one to the last one
ArrowLeft / j       - scroll items left (usefull for long names)
ArrowRight / l      - scroll items right (usefull for long names)
Space               - select current item
TAB                 - switch from left to right panel and vice versa
Return              - enter directory
F2/2                - refresh focused pane (reload files/dirs)
F3/3                - view current file or DataPower configuration
F4/4                - edit file
F5/5                - copy selected (or current if none selected) directories and files
F7/7                - create directory
DEL/d               - delete selected (or current if none selected) directories and files
/                   - find string
n                   - find next string
p                   - find previous string
f                   - filter shown items by string
.                   - enter location (full path) for local file system
s                   - auto-synchronize selected directories (local to dp)
q                   - quit
any-other-char      - show help (+ hex value of key pressed visible in status bar)

Navigational keys (except Left/Right can be used in combination with Shift for selections):
PgUp  Up  PgDn
Left Down Right
Home      End

Alternative keys:
u i o
j k l
a   z

Custom Viewer/Editor commands:
dpcmder configuration is saved to ~/.dpcmder/config.json where commands used for
calling viewer and editor are set. By default these are "less" and "vi" but could
be any commands although edit command should not be started in background (for
example: a new tab in editor which is already open) for editing to work as expected.

SOMA (+ AMP) vs REST:
SOMA and AMP interfaces have one shortcoming - you can't see domain list if you
don't have proper rights. With REST you can get domain list without any credentials.

TODO:

- add diff for files
- add default value for input questions (set it after first user selection), for example:
  - Are you sure you want to disable sync mode [y/n] (): y
	- Are you sure you want to disable sync mode [y/n] (y):
- handle border cases (open domain for which you don't have rights...)
- tests
- docs
`

func Show() {
	extprogs.View("Help", []byte(Help))
}
