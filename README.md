# hoi4geoparser

Used to generate image based terrain maps from hoi4 data. Originally written by malashin.

### Languages:
- [English](https://github.com/ICodeMaster/hoi4geoparser/blob/master/README.md)
- [Русский](https://github.com/ICodeMaster/hoi4geoparser/blob/master/README-RU.md)

### Install hoi4geoparser
- Download the binary from the [releases page](https://github.com/ICodeMaster/hoi4geoparser/releases)
- Run the geoparser once to generate a config file.
- Edit the config file to point to the mod or game path
- Run the geoparser and input the maps to generate when prompted

### Add files for custom map modes to the mod
- Copy the files from the mod into your mod
- Drag the folders inside mod_path (found in the tool's directory) into your mod path. That's it!

____

The project is distributed under the MIT license, read more in the [LICENSE](https://github.com/ICodeMaster/hoi4geoparser/blob/master/LICENSE)

### Reference
global.states -> All of the states in the map function

find_color_graph = yes -> Creates country colors so no neighbors share a color

set_state_flag = mapmode_state_hashed_visible -> State has hashed texture

set_state_flag = mapmode_state_visible -> State has normal texture visible

state_frame_number_hashed -> Frame (Color) for hashed state texture

state_frame_number -> Frame (Color) for normal state texture

calculate_country_center_point_quick_all = yes -> Find the rough centerpoint of every nation

set_country_flag = mapmode_shield_visible -> Show shield for a nation

set_country_flag = mapmode_shield_use_capital -> Use capital state for country flag instead of centerpoint

set_state_flag = info_text_visible -> Show info text
