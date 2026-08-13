# DEROtui - Change Password
# Shows the 3-field change password form with password strength indicator

Output change-password.gif
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

# Open change password
Type "p"
Sleep 1s

# Current password
Type "password123"
Sleep 0.5s

# Tab to New Password
Tab
Sleep 0.5s

# Type new password (strong, triggers strength indicator)
Type "MyNewStr0ngP@ss42"
Sleep 1s

# Tab to Confirm
Tab
Sleep 0.5s

# Confirm new password
Type "MyNewStr0ngP@ss42"
Sleep 1s

# Submit
Enter
Sleep 2s

# Close wallet
Escape
Sleep 1s
Type "q"
