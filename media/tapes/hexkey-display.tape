# DEROtui - Hex Key Display
# Shows the 64-character hex key split across two lines in a bordered box

Output hexkey-display.gif
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

# Unlock wallet
Type "password123"
Sleep 0.5s
Enter
Sleep 2s

# View hex key
Type "k"
Sleep 3s

# Continue (close hex key display)
Enter
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
