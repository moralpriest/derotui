# DEROtui - XSWD Server Toggle
# Shows pressing X on the dashboard to toggle the XSWD server on/off
# Green dot indicator appears when running

Output xswd-toggle.gif
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

# Switch to Page 2 (Advanced)
Right
Sleep 1s

# Navigate down to XSWD action (5 items down)
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s
Down
Sleep 0.8s

# Toggle XSWD ON - green dot appears
Type "x"
Sleep 2s

# Toggle XSWD OFF - dot returns to muted
Type "x"
Sleep 1s

# Close wallet
Escape
Sleep 1s
Type "q"
