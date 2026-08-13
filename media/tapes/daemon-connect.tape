# DEROtui - Daemon Connection
# Shows the /connect flow with network radio buttons and address input
# Cycles through Mainnet/Testnet/Simulator, types address, attempts connect

Output daemon-connect.gif
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

# Open daemon connect
Type "/connect"
Enter
Sleep 1s

# Cycle through networks: Mainnet -> Testnet -> Simulator -> back to Mainnet
Right
Sleep 1s
Right
Sleep 1s
Left
Sleep 1s

# Type custom address
Type "192.168.1.100"
Sleep 1s

# Attempt connection
Enter
Sleep 2s

# Close (will error since no daemon, but shows input flow)
Escape
Sleep 1s
Type "q"
