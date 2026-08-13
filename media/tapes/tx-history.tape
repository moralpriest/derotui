# DEROtui - Transaction History
# Shows the custom color-coded table (green IN/MINED, red OUT)
# Navigates through rows with selection highlighting

Output tx-history.gif
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

# Navigate through transactions
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 1s

# Close history
Escape
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
