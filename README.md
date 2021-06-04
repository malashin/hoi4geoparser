# hoi4geoparser

Used to generate image based terrain maps from hoi4 data. Originally written by malashin.

### Languages:
- English
- Русский

### Install hoi4geoparser
1. **Install Go** 
- Open url in browser - <https://golang.org/dl/>
- Select the compiled package for your OS - Windows, Linux, Mac OS
- Install it on your PC.

2. Download Repo
- You can download this repository as a [zip archive](https://github.com/ICodeMaster/hoi4geoparser/archive/refs/heads/master.zip)
- Clone it using git:
``git clone https://github.com/ICodeMaster/hoi4geoparser.git``

### Using:
1. Open the main.go in convenient code editor (Notepad++, VS Code, Sublime). 
2. Replace the path to the mod with the path on your pc.

Example: ``var modPath = "C:/Users/User/Documents/your-mod"`` 

3. Open the Terminal(command line or cmd) and write it down ``go run .\main.go``
4. Insert into your mod:
- Copy the ``state_images`` folder into ``gfx/interface/custom_map_modes/`` in your mod
- Copy the ``custom_states_generated_state_images.gfx`` in ``state_images`` into the ``/interface/`` folder your mod. 
- Copy the ``state_centers_on_actions.txt`` into ``common/on_actions``
