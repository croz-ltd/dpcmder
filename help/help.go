// Package help contains help message listing all commands available in
// dpcmder and how to setup external commands. Help message can be shown from
// command line (if flag "-help" is given) or inside dpcmder application when
// key "h" or any unrecognized key is pressed.
package help

// Help contains dpcmder in-program help shown when 'h' or unrecognized key is pressed.
const Help = `dpcmder Help

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
ArrowLeft / j       - scroll items left (useful for long names)
ArrowRight / l      - scroll items right (useful for long names)
Space               - select current item
TAB                 - switch from left to right panel and vice versa
Return              - enter directory
F2/2                - refresh focused pane (reload files/dirs)
F3/3                - view current file or DataPower configuration
                      (see "Custom external commands" below)
F4/4                - edit file
                      (see "Custom external commands" below)
F5/5                - copy selected (or current if none selected) directories and files
                    - if selected is DataPower domain create export of domain
                    - if selected is DataPower configuration create export of whole appliance (TODO)
F7/7                - create directory
F8/8                - create empty file
                    - create new DataPower configuration
F9/9                - clone current DataPower configuration under new name
DEL/x               - delete selected (or current if none selected) directories and files
                    - delete DataPower configuration
                    - delete DataPower object
d                   - diff current files/directories
                      (should be "blocking" - see "Custom external commands" below)
                    - diff changes on modified DataPower object (SOMA only)
/                   - find string
n                   - find next string
m                   - show all status messages saved in history
p                   - find previous string
f                   - filter shown items by string
.                   - enter location (full path) for local file system
s                   - auto-synchronize selected directories (local to dp)
S                   - save running DataPower configuration (SOMA only)
0                   - toggle DataPower view from filestore view to object view
                    - when using SOMA access changed objects are marked and object
                      changes can be shown using diff (d key)
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

Custom external commands (Viewer/Editor/Diff):
dpcmder configuration is saved to ~/.dpcmder/config.json where commands used for
calling external commands are set. By default these are "less", "vi" and "ldiff"
but could be any commands. All of those commands should be started in foreground
and should wait for user's input to complete. For example for viewer "less" or
"more" can be used while "cat" will not work. For file comparison normal "diff"
command can be used as work around but "blocking" diff command should be used
like vimdiff. Custom ldiff script which combines diff and less commands can be
easily prepared (something like:
 'diff "$1" "$2" | less'
or for more fancy colored output you can use something like:
 'diff -u -r --color=always "$1" "$2" | less -r').

SOMA (+ AMP) vs REST:
SOMA and AMP interfaces have one shortcoming - you can't see domain list if you
don't have proper rights. With REST you can get domain list without any credentials.
Some new features added are SOMA-only. For example with REST you can't see
changed objects or compare persisted DataPower object configuration to saved
configuration.

TODO:

- save changes
- add domain/appliance export[/import?]
- add creation of files/directories/domains/objects
- handle border cases (open domain for which you don't have rights...)

(to show dpcmder usage help use "-h" flag instead of "-help" flag)
`
