# DEROtui - QR Code Display
# Shows the Unicode half-block QR code with wallet address below

Output qrcode.gif
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

# View QR code
Type "y"
Sleep 3s

# Close QR
Escape
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
