# DEROtui - Send to Integrated Address
# Pastes an integrated address (deroi...) and shows auto-parsed
# Payment ID (✓ from address) + Message fields

Output integrated-addr-send.gif
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

# Open send form
Type "s"
Sleep 1s

# NOTE: Replace this address with a real generated deroi... address
# Run payment-request.tape first to generate one,
# then copy the address here
Type "deroi1qyre7td6x9r88y4cavdgpv6k7lvx6j39lfsx420hpvh3ydpcrtxrxqtest"
Sleep 2s

# Tab to Amount (auto-parsed Payment ID shows ✓ from address)
Tab
Sleep 1s

# Tab to Ringsize
Tab
Sleep 1s

# Tab to Message field (note: auto-filled if address is an integrated address)
Tab
Sleep 1s

# Tab to Payment ID (note: shows ✓ from address indicator)
Tab
Sleep 1s

# Close
Escape
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
