# DEROtui - Create Wallet Flow
# Shows the wallet creation form with password fields

Output create-flow.gif
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

# Open /create
Type "/create"
Enter
Sleep 1s

# wallet.db is pre-filled, Tab to password field
Tab
Sleep 0.5s

# Type password
Type "password123"
Sleep 0.5s

# Tab to confirm password
Tab
Sleep 0.5s

# Type confirm password
Type "password123"
Sleep 0.5s

# Enter to confirm creation (triggers network selection)
Enter
Sleep 2s

# Network selection shows Mainnet as default, press Enter to confirm
Enter
Sleep 2s

# Close seed display (goes to dashboard)
Escape
Sleep 1s

# Exit
Type "q"
