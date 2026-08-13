# DEROtui - Command Menu Showcase
# Cycles through all slash commands on the welcome page

Output command-menu.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

# Launch from /tmp to avoid auto-opening any wallet
Hide
Type "~/temp/derotui/derotui"
Enter
Sleep 4s
Show
Sleep 1s

# Open command menu with /
Type "/"
Sleep 1s

# /open (already highlighted)
Sleep 1s

# /create
Down
Sleep 1s

# /restore
Down
Sleep 1s

# /themes
Down
Sleep 1s

# /connect
Down
Sleep 1s

# /debug
Down
Sleep 1s

# /exit
Down
Sleep 1s

# Back to /open (completes the loop)
Down
Sleep 2s

# Exit
Type "q"
