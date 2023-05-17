// Package help contains help message listing all commands available in
// dpcmder and how to setup external commands. Help message can be shown from
// command line (if flag "-help" is given) or inside dpcmder application when
// key "h" or any unrecognized key is pressed.
package help

// Version (shoud be git tag), Platform & Time contains application information
// which can be set during build time (using ldflags -X) and is shown
// when "-v" flag is passed to dpcmder.
var (
	Version   = "development"
	Platform  = "local"
	BuildTime = ""
)

// Help contains dpcmder in-program help shown when 'h' or unrecognized key is pressed.
const Help = `dpcmder Help

ArrowUp / i          - move one item up
ArrowDown / k        - move one item down
Shift+ArrowUp / I    - select a current item and move one item up (can use Alt instead of Shift)
Shift+ArrowDown / K  - select a current item and move one item down (can use Alt instead of Shift)
PgUp / u             - move a page of items up
PgDown / o           - move a page of items down
Shift+PgUp / U       - select a current item and move a page of items up
Shift+PgDown / O     - select a current item and move a page of items down
Home / a             - move to the first item
End / z              - move to the last item
Shift+Home / A       - move to the first item and select all items from the current one to the first one
Shift+End / Z        - move to the last item and select all items from the current one to the last one
ArrowLeft / j        - scroll items left (useful for long names)
ArrowRight / l       - scroll items right (useful for long names)
Shift+ArrowLeft / J  - navigate to the previous view for the current side
Shift+ArrowRight / L - navigate to the next view for the current side
H                    - show view history list - can jump to any view in the current history
Space                - select current item
TAB                  - switch from left to right panel and vice versa
Return               - enter directory
F2/2                 - refresh focused pane (reload files/dirs)
F3/3                 - view current file, DataPower configuration, DataPower
                       object, DataPower status (or all statuses of same class)
                       (see "Custom external commands" below)
F4/4                 - edit file
                       (see "Custom external commands" below)
F5/5                 - copy the selected (or current if none selected) directories and files
                     - if DataPower domain is selected create an export of the domain
                     - if DataPower configuration is selected create an export of
                       the whole appliance (SOMA only)
                     - in DataPower object configuration mode copy DataPower
                       object to file or copy file with proper object configuration
                       to DataPower object (XML/JSON, depending on REST/SOMA
                       management interface used)
F7/7                 - create directory
                     - create a new DataPower domain
F8/8                 - create an empty file
                     - create a new DataPower configuration
F9/9                 - clone a current DataPower configuration under a new name
                     - clone a current DataPower object under new name
DEL/x                - delete selected (or current if none selected) directories and files
                     - delete a DataPower configuration
                     - delete a DataPower object
d                    - diff current files/directories
                       (should be "blocking" - see "Custom external commands" below)
                     - diff changes on modified DataPower object (SOMA only)
/                    - find string
n                    - find next string
N                    - find previous string
f                    - filter visible items by a given string
m                    - show all status messages saved in the history
.                    - enter a location (full path) for the local file system
s                    - auto-synchronize selected directories (local to DataPower)
S                    - save a running DataPower configuration
B                    - create and copy a secure backup of the appliance
e                    - run exec command on current or selected cfg file(s)
0                    - cycle between different DataPower view modes
                       filestore mode view / object mode view / status mode view
                     - when using SOMA access changed objects are marked and object
                       changes can be shown using diff (d key)
?                    - show status information for the current DataPower object,
                       local directory or local file
P                    - show DataPower policy for the current DataPower object
                       (can be used only on service, policy, matches, rules & actions)
                       exports the current DataPower object, analyzes it and
                       shows service, policy, matches, rules and actions for
                       the object
h                    - show help
q                    - quit
any-other-char       - show help (+ hex value of the key pressed visible in the status bar)

Navigational keys (except Left/Right can be used in combination with Shift for selections):
PgUp  Up  PgDn
Left Down Right
Home      End

Alternative keys:
u i o
j k l
a   z

DataPower view mode (filestore / object / status):
You can cycle though 3 available DataPower view mode (when focus is on the
DataPower view).
- filestore view mode: file maintenance, domain or appliance backup.
- object view mode: view DataPower objects and make quick configuration changes.
- status view mode: view DataPower statuses and flush xsl & document caches.

Custom external commands (Viewer/Editor/Diff):
dpcmder configuration is saved to ~/.dpcmder/config.json where commands used for
calling external commands are set. By default, these are "less", "vi" and "diff"
but could be any commands. All of those commands should be started in the
foreground and should wait for the user's input to complete. For example for
viewers "less" or "more" can be used while "cat" will not work. For file
comparison, normal "diff" command can be used as a workaround but "blocking" diff
command should be used (like vimdiff). Custom ldiff script which combines diff
and "less" commands can be easily prepared using something like:
  #!/bin/bash
  diff "$1" "$2" | less
or for the fancier colored output you can use something like:
  #!/bin/bash
  diff -u -r --color=always "$1" "$2" | less -R

SOMA (+ AMP) vs REST:
SOMA and AMP interfaces have one shortcoming - you can't see domain list if you
don't have proper rights. With REST you can get domain list without any credentials.
Some of the DataPower information is not available using REST interface.
Some new features added are SOMA-only. For example, with REST you can't compare
persisted DataPower object configuration to saved configuration.

TODO:
- add domain/appliance import?
- add creation of new DataPower objects
  (should be able to show/create all classes of objects, even ones without
  object instances)
- add deletion of a domain (with maybe 2 confirmation dialogs: are you sure?;
  are you really sure?)

(to show dpcmder usage help use "-h" flag instead of "-help" flag)
`
