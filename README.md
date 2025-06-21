# kscribbler

Sync your Kobo highlights to [hardcover.app](https://hardcover.app)!

*K* - Kobo|kepub
*Scribe* - Someone who writes stuff to a new location

ps. add me on hardcover 🤓 [https://hardcover.app/@countmancy](https://hardcover.app/@countmancy) 

## Installation

## Upgrade

Same process as installation for now. You should not need to re-add your hardcover token.

I would like to make this over-the-air (OTA) updateable in the future, but the ways I've tested so far have been too hands-on techy for general users.

## Usage

Once installed `nickelmenu` should have a new entry in the right corner of reading view that will trigger a sync when clicked.
To save some battery and resources the sync is only ran against the current/last opened book.

The quotes are uploaded to hardcover.app based on the book's ISBN. See the note below or the troubleshooting section if quotes are not being uploaded.

- If your book doesn't have an ISBN saved to the kobo device's database you can highlight the book's ISBN on its copyright page and `kscribbler` will attempt to parse it and save it to the book's metadata.
  - Can be useful for sideloaded books
  - Currently is uploading the isbn quote to hardcover.app - this is fixable

## Troubleshooting
- Logs are stored in `/mnt/onboard/.adds/kscribbler/kscribbler.log`
- If you are having issues with the quotes not being uploaded, check that hardcover.app has an edition for the ISBN.
- For the savy the sqlite db is located at`/mnt/onboard/.kobo/KoboReader.sqlite`
  - You can modify this database to add an ISBN to a book if you know what you're doing

## Contributing
- File bug reports
- Submit pull requests
- Suggest improvements
- Share the project with others if you find it useful

## Check out my other ebook-related projects
  - [hardcover-quotes](https://github.com/GianniBYoung/hardcover-quotes) -> Display hardcover *reading journal* quotes in your terminal or on a [trmnl e-ink display](https://usetrmnl.com) 
    - Don't have a trmnl? Here is my referral link: [trmnl.com](https://usetrmnl.com?ref=GianniBYoung)
  - [simpleISBN](https://github.com/GianniBYoung/simpleISBN) -> A simple library to parse and convert ISBNs
