# DEROtui - Transaction Details
# Shows section-based layout (Overview, Block, Transaction, Ports)
# Cycles through all copy keybindings with flash feedback

Output tx-details.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

Hide
Type "~/temp/derotui/derotui --wallet-file ~/temp/mainnet_simulator/wallet_0.db"
Enter
Sleep 4s
Show
Sleep 2s

# Unlock wallet
Type "password"
Sleep 0.5s
Enter
Sleep 3s

# Open history
Type "h"
Sleep 2s

# Select first transaction and open details
Enter
Sleep 2s

# Cycle through copy keybindings with flash feedback
Type "t"
Sleep 1s
Type "h"
Sleep 1s
Type "s"
Sleep 1s
Type "d"
Sleep 1s
Type "p"
Sleep 1s
Type "m"
Sleep 1s
Type "i"
Sleep 1s

# Close details
Escape
Sleep 1s

# Close history
Escape
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
