# DEROtui - Send DERO
# Fills out the send form and submits a transaction

Output send-dero.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

Hide
Type "~/temp/derotui/derotui --wallet-file ~/temp/derotui/wallet.db"
Enter
Sleep 2s
Show
Sleep 2s

# Type password and enter wallet
Type "password"
Sleep 0.5s
Enter
Sleep 2s

# Open send form
Type "s"
Sleep 0.5s

# Type recipient address
Type "dero1qyre7td6x9r88y4cavdgpv6k7lvx6j39lfsx420hpvh3ydpcrtxrxqga4mp52"
Sleep 1s

# Tab to amount
Tab
Sleep 1s
Type "127"
Sleep 1s

# Tab to ringsize (16 is default, Right arrow 3 times to reach 128)
Tab
Sleep 1s
Right
Sleep 0.5s
Right
Sleep 0.5s
Right
Sleep 1s

# Tab to message
Tab
Sleep 1s
Type "This new TUI wallet is awesome!"
Sleep 1s

# Tab to destination port
Tab
Sleep 1s
Type "13"
Sleep 1s

# Tab to Send button, then Enter
Tab
Sleep 1s
Enter
Sleep 7s

# Cancel and exit
Escape
Sleep 1s
Type "q"
