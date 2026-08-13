# DEROtui - Theme Showcase
# Cycles through all 7 themes on the welcome page

Output themes-showcase.gif
Set FontSize 14
Set Width 900
Set Height 700
Set Framerate 24

# Launch from /tmp to avoid auto-opening any wallet
Hide
Type "~/temp/derotui/derotui"
Enter
Sleep 4s
Show
Sleep 1s

# Open themes menu
Type "/themes"
Enter
Sleep 1s

# Crimson Dark
Down
Sleep 1.2s

# Gruvbox Dark
Down
Sleep 1.2s

# Matrix
Down
Sleep 1.2s

# Neon
Down
Sleep 1.2s

# Neon Pink
Down
Sleep 1.2s

# Solarized Dark
Down
Sleep 1.2s

# Back to Amber CRT (completes the loop)
Down
Sleep 2s

# Exit
Type "q"
