# DEROtui - Dashboard Navigation
# Opens wallet, navigates through all quick actions on both pages

Output dashboard-nav.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

Hide
Type "~/temp/derotui/derotui --wallet-file ~/temp/derotui/wallet.db"
Enter
Sleep 4s
Show
Sleep 1s

# Type password
Type "test"
Sleep 0.5s
Enter
Sleep 3s

# Navigate Page 1 actions (Core)
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s

# Switch to Page 2 (Advanced)
Right
Sleep 1s

# Navigate Page 2 actions
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 1.5s

# Exit
Type "q"
