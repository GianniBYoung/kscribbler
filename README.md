# kscribbler

Sync your Kobo highlights to [hardcover.app](https://hardcover.app)!

*K* - Kobo|kepub
*Scribe* - Someone who writes stuff to a new location

ps. add me on hardcover 🤓 [https://hardcover.app/@countmancy](https://hardcover.app/@countmancy?referrer_id=28377) 

## Installation
1. Download and install [nickelmenu](https://pgaskin.net/NickelMenu)
2. Download the latest release of `kscribbler` from the [releases page](https://github.com/GianniBYoung/kscribbler/releases/latest)
3. Install `kscribbler` on your Kobo:
   - Connect your Kobo to your computer and mount the device
   - Navigate to the `.kobo` directory on your Kobo (might need to show hidden files)
   - Copy the `KoboRoot.tgz` into this directory and unmount the device safely. This will trigger a reboot and upgrade.
4. Remount the device and navigate to the `.adds/kscibbler` directory and add your hardcover token.

## Upgrade

Same process as installation for now. You should not need to re-add your hardcover token.

I would like to make this over-the-air (OTA) updatable in the future, but the ways I've tested so far have been too hands-on techy for general users.

## Usage

Once installed `nickelmenu` should have a new entry in the right corner of reading view that will trigger a sync when clicked.
To save some battery and resources the sync is only ran against the current/last opened book.

The quotes are uploaded to hardcover.app based on the book's ISBN. See the note below or the troubleshooting section if quotes are not being uploaded.

- If your book doesn't have an ISBN saved to the kobo device's database you can highlight the book's ISBN on its copyright page and `kscribbler` will attempt to parse it and save it to the book's metadata.
  - Can be useful for sideloaded books

## Troubleshooting
- Logs are stored in `/mnt/onboard/.adds/kscribbler/kscribbler.log`
- If you are having issues with the quotes not being uploaded, check that hardcover.app has an edition for the ISBN.
- This project depends on the `KoboReader.sqlite` database to be up to date. This typically means a successful sync is required for the DB to actually write from memory to disk.
  - This leads to some annoying timing issues. I would recommend running the program after a sync and reboot if you are having trouble uploading quotes.
- For the savvy the sqlite db is located at`/mnt/onboard/.kobo/KoboReader.sqlite`
  - You can modify this database to add an ISBN to a book if you know what you're doing
  - `telnet/ssh` into the kobo is possible and allows for manually running `kscribbler` if so desired

## Contributing
- File bug reports
- Submit pull requests
- Suggest improvements
- Share the project with others if you find it useful

## Check out my other ebook-related projects
  - [hardcover-quotes](https://github.com/GianniBYoung/hardcover-quotes) -> Display hardcover *reading journal* quotes in your terminal or on a [trmnl e-ink display](https://usetrmnl.com) 
  - [simpleISBN](https://github.com/GianniBYoung/simpleISBN) -> A simple library to parse and convert ISBNs
