# kscribbler

Sync your Kobo highlights to [hardcover.app](https://hardcover.app)!

Here is an [example](https://hardcover.app/books/crooked-kingdom/journals/@countmancy?referrer_id=28377) for the book Crooked Kingdom by Leigh Bardugo:



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
4. Remount the device and navigate to `/mnt/onboard/.adds/kscribbler`.
  - If you don't see this directory check for `/mnt/onboard/.adds/kscribbler`
5. Copy the example config file and add your token:
    - `cp config.env.example config.env`
    - `vi config.env`

## Upgrade

Same process as installation for now. You should not need to re-add your hardcover token.

I would like to make this over-the-air (OTA) updatable in the future, but the ways I've tested so far have been too hands-on techy for general users.

## Usage

Once installed `nickelmenu` should have a new entry in the right corner of reading view that will trigger a sync when clicked.

This will trigger `kscribbler` to generate its database of quotes and upload them to hardcover.app.
WARNING: This will attempt to upload all quotes from all books on the device. If you want to tame this you will need to manipulate the `kscribblerdb` yourself. More info below.

The quotes are uploaded to hardcover.app based on the book's ISBN. See the note below or the troubleshooting section if quotes are not being uploaded.

- If your book doesn't have an ISBN saved to the kobo device's database you can highlight the book's ISBN on its copyright page and `kscribbler` will attempt to parse it and save it to the book's metadata.
  - A note with `kscrib:<you isbn number with no angle brackets` can be added anywhere in the book to manually set the ISBN
  - Can be useful for sideloaded books

## Troubleshooting
- Logs are stored in `/mnt/onboard/.adds/kscribbler/kscribbler.log`
- If you are having issues with the quotes not being uploaded, check that hardcover.app has an edition for the ISBN.

## Advanced Usage
- The database of quotes is stored at `/mnt/onboard/.adds/kscribbler/kscribblerdb`
- This is a sqlite database with two tables: `books` and `quotes`
- You can manipulate this database directly if you want to control what gets uploaded by setting `kscribbler_uploaded` to `1` for quotes you don't want uploaded
- `telnet/ssh` into the kobo is possible and allows for manually running `kscribbler` if so desired
- From the main Kobo screen you can open nickelmenu and `Toggle Visibility of Kscribbler Options` to run the following commands:
  - `kscribbler --init` will initialize the database but not upload anything
  - `kscribbler --mark-all-as-uploaded` will initialize the database, mark all found quotes as upload but will not upload anything
    - Useful for testing/migrating
  - The initial output is displayed but truncated. Full output is in the log file


## Contributing
- Star the repository ⭐
- File bug reports
- Submit pull requests
- Suggest improvements
- Share the project with others if you find it useful

## Check out my other ebook-related projects
  - [hardcover-quotes](https://github.com/GianniBYoung/hardcover-quotes) -> Display hardcover *reading journal* quotes in your terminal or on a [trmnl e-ink display](https://usetrmnl.com) 
  - [simpleISBN](https://github.com/GianniBYoung/simpleISBN) -> A simple library to parse and convert ISBNs
