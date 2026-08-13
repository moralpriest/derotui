# DEROtui - Seed Phrase Display
# Shows the 25-word card grid (5x5) with purple bold words and numbered cards

Output seed-display.gif
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

# View seed
Type "v"
Sleep 3s

# Continue (close seed display)
Enter
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
