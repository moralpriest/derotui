# DEROtui - Payment Request (Integrated Address)
# Creates an integrated address with destination port, message, and amount
# Shows auto-QR on success

Output payment-request.gif
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

# Open payment request
Type "r"
Sleep 1s

# Destination Port
Type "1337"
Sleep 1s

# Tab to Message
Tab
Sleep 0.5s

# Message
Type "Thanks for the DERO!"
Sleep 1s

# Tab to Amount
Tab
Sleep 0.5s

# Amount
Type "42"
Sleep 1s

# Tab to Generate button
Tab
Sleep 0.5s

# Generate integrated address
Enter
Sleep 2s

# Auto-QR is now displayed
Sleep 2s

# Close QR
Escape
Sleep 1s

# Close payment request
Escape
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
