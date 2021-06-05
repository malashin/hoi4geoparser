# hoi4geoparser

Используется для создания отдельных карт местности на основе данных hoi4. Первоначально написано malashin.

### Языки:
- [English](https://github.com/ICodeMaster/hoi4geoparser/blob/master/README.md)
- [Русский](https://github.com/ICodeMaster/hoi4geoparser/blob/master/README-RU.md)

### Установка hoi4geoparser
1. **Установка Go** 
- Открыть URL - адрес в браузере - <https://golang.org/dl/>
- Выберите установочный пакет для вашей ОС - Windows, Linux, Mac OS
- Установите его на свой компьютер.

2. **Загрузка данного репозитория**
- Вы можете загрузить этот репозиторий в виде [zip-архива](https://github.com/ICodeMaster/hoi4geoparser/archive/refs/heads/master.zip)
- Клонируйте его с помощью git:
`git clone https://github.com/ICodeMaster/hoi4geoparser.git`

### Использование:
1. Откройте main.go в удобном редакторе кода (Notepad++, VS Code, Sublime). 
2. Замените путь к моду на путь на вашем компьютере.

Пример: `var modPath = "C:/Users/User/Documents/your-mod"` 

3. Откройте Терминал(Командная строка или cmd) и напишите в нем `go run .\main.go`
4. Вставьте в свой мод:
- Скопируйте папку `state_images` в `gfx/interface/custom_map_modes/` вашего мода.
- Скопируйте `custom_states_generated_state_images.gfx` в `state_images` в папку `/interface/` вашего мода. 
- Скопируйте `state_centers_on_actions.txt` в `common/on_actions`

____
Переведено [sepera_okeq](https://github.com/Sepera-okeq).

Проект распространяется под лицензией MIT, подробнее читайте в файле [ЛИЦЕНЗИЯ](https://github.com/ICodeMaster/hoi4geoparser/blob/master/LICENSE)
