# hoi4geoparser

Используется для создания отдельных карт местности на основе данных hoi4. Первоначально написано malashin.

### Языки:
- [English](https://github.com/ICodeMaster/hoi4geoparser/blob/master/README.md)
- [Русский](https://github.com/ICodeMaster/hoi4geoparser/blob/master/README-RU.md)

### Установка и использование hoi4geoparser
- Скачать готовые билды со [страницы GitHub релизов](https://github.com/ICodeMaster/hoi4geoparser/releases)
- Запустите Geoparser один раз, чтобы сгенерировать конфигурационный файл.
- Отредактируйте файл конфигурации (указать путь к моду или игре, и т.д настройки)
- Запустите hoi4geoparser.

### Добавление сгенерированных файлов для пользовательских режимов карты в вашем моде:
- Скопируйте папку ``state_images`` в``gfx/interface/custom_map_modes/`` вашего мода.
- Скопируйте файл ``custom_states_generated_state_images.gfx`` в ``state_images`` в папку ``/interface/`` вашего мода. 
- Скопируйте ``state_centers_on_actions.txt`` в ``common/on_actions``

____
Переведено [sepera_okeq](https://github.com/Sepera-okeq).

Проект распространяется под лицензией MIT, подробнее читайте в файле [ЛИЦЕНЗИЯ](https://github.com/ICodeMaster/hoi4geoparser/blob/master/LICENSE)
