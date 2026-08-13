# DEROtui - Restore from Hex Key
# Shows 64-char hex key input, then password form

Output restore-key.gif
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

# Navigate to "From Key"
Down
Enter
Sleep 1s

# Type 64-char hex key
Type "a3f8c2d1e7b4096f5a8d3c7e1b9f4a6d2e8c5b0f7a3d9e1c6b4f8a2d5e7c0b3f"
Sleep 1s

# Enter to proceed to password form
Enter
Sleep 2s

# Cancel and exit
Escape
Sleep 1s
Type "q"
