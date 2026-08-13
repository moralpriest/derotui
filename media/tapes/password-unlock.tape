# DEROtui - Password Unlock
# Shows the styled unlock prompt with wallet path, version in border, password input

Output password-unlock.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

Hide
Type "~/temp/derotui/derotui --wallet-file ~/temp/derotui/wallet.db"
Enter
Sleep 4s
Show
Sleep 2s

# Type password
Type "password123"
Sleep 1s

# Enter to unlock
Enter
Sleep 2s

# Close wallet
Escape
Sleep 1s
Type "q"
