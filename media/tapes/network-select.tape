# DEROtui - Network Selection
# Shows the network picker, cycles through Mainnet/Testnet/Simulator

Output network-select.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

Hide
Type "~/temp/derotui/derotui --restore-deterministic-wallet --electrum-seed \"pistons tacit archer lyrics second plus molten alkaline lunar terminal brunt tyrant ruling archer smidgen goodbye utopia tadpoles cynical summon melting exotic magically hefty summon\""
Enter
Sleep 5s
Show
Sleep 1s

# Navigate: Mainnet -> Testnet -> Simulator -> back to Mainnet
Down
Sleep 1s
Down
Sleep 1s
Up
Sleep 1s

# Confirm Mainnet
Enter
Sleep 2s

# Close
Escape
Sleep 1s
Type "q"
