# DEROtui - Restore from Seed
# Shows seed input with 25 valid Monero words, then password form

Output restore-seed.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

Hide
Type "~/temp/derotui/derotui"
Enter
Sleep 4s
Show
Sleep 1s

# Open restore submenu
Type "/restore"
Enter
Sleep 1s

# "From Seed" is already highlighted, press Enter
Enter
Sleep 1s

# Type 25-word seed
Type "pistons tacit archer lyrics second plus molten alkaline lunar terminal brunt tyrant ruling archer smidgen goodbye utopia tadpoles cynical summon melting exotic magically hefty summon"
Sleep 1s

# Enter to proceed to password form
Enter
Sleep 2s

# Cancel and exit
Escape
Sleep 1s
Type "q"
