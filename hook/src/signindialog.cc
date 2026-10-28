#include <QGuiApplication>
#include <QPixmap>
#include <QScreen>

#include <NickelHook.h>
#include <qrcodegen.h>

#include "cli.h"
#include "signindialog.h"

SignInDialog::SignInDialog() : QFrame() {
  layout = new QVBoxLayout(this);
  layout->setContentsMargins(0, 0, 0, 0);

  status = new QLabel("Connecting to Hardcover...", this);
  status->setAlignment(Qt::AlignCenter);
  status->setWordWrap(true);
  layout->addWidget(status, 1, Qt::AlignCenter);

  dialog = N3DialogFactory__getDialog(this, true);
  N3Dialog__setTitle(dialog, "Sign in to Hardcover");
  MainWindowController__pushView(MainWindowController__sharedInstance(), dialog);
  QObject::connect(dialog, SIGNAL(closeTapped()), dialog, SLOT(deleteLater()));
  dialog->show();

  CLI *cli = CLI::oauthRequest();
  QObject::connect(cli, &CLI::finished, this, &SignInDialog::requestFinished);
}

void SignInDialog::fail(const QString &message) {
  ConfirmationDialogFactory__showErrorDialog("Kscribbler", message);
  dialog->deleteLater();
}

void SignInDialog::requestFinished(const QJsonObject &result) {
  if (result.value("status").toString() != "ok") {
    fail(result.value("message").toString());
    return;
  }

  deviceCode = result.value("device_code").toString();
  QString link = result.value("verification_uri_complete").toString();
  QString uri = result.value("verification_uri").toString();
  if (link.isEmpty()) {
    link = uri;
  }

  layout->removeWidget(status);
  layout->addStretch(1);

  QLabel *qr = new QLabel(this);
  qr->setPixmap(QPixmap::fromImage(buildQrCode(link, QGuiApplication::primaryScreen()->size().width() / 2)));
  layout->addWidget(qr, 0, Qt::AlignCenter);

  QLabel *instructions = new QLabel(
      QString("<br>Scan this QR code<br><br>— or —<br><br>visit <b>%1</b> and enter<br>").arg(QString(uri).remove("https://")),
      this);
  instructions->setAlignment(Qt::AlignCenter);
  layout->addWidget(instructions, 0, Qt::AlignCenter);

  QLabel *code = new QLabel(QString("<h1>%1</h1>").arg(result.value("user_code").toString()), this);
  code->setAlignment(Qt::AlignCenter);
  layout->addWidget(code, 0, Qt::AlignCenter);

  status->setText("Approve on Hardcover, then tap Continue.");
  layout->addWidget(status, 0, Qt::AlignCenter);

  N3ButtonLabel *button = construct_N3ButtonLabel(this);
  button->setProperty("primaryButton", true);
  button->setText("Continue");
  layout->addWidget(button, 0, Qt::AlignCenter);
  QObject::connect(button, SIGNAL(tapped(bool)), this, SLOT(continueTapped()));

  layout->addStretch(1);
}

void SignInDialog::continueTapped() {
  if (busy) {
    return;
  }
  busy = true;
  status->setText("Checking...");

  CLI *cli = CLI::oauthSet(deviceCode);
  QObject::connect(cli, &CLI::finished, this, &SignInDialog::setFinished);
}

void SignInDialog::setFinished(const QJsonObject &result) {
  busy = false;
  QString state = result.value("status").toString();

  if (state == "ok") {
    toast("Kscribbler is connected to Hardcover");
    dialog->deleteLater();
  } else if (state == "pending") {
    status->setText(result.value("message").toString());
  } else {
    fail(result.value("message").toString());
  }
}

QImage SignInDialog::buildQrCode(const QString &text, int targetSize) {
  constexpr int maxVersion = 6;
  constexpr size_t bufLen = qrcodegen_BUFFER_LEN_FOR_VERSION(maxVersion);
  uint8_t qrcode[bufLen];
  uint8_t tempBuffer[bufLen];

  QByteArray utf8 = text.toUtf8();
  if (!qrcodegen_encodeText(utf8.constData(), tempBuffer, qrcode, qrcodegen_Ecc_MEDIUM, qrcodegen_VERSION_MIN,
                            maxVersion, qrcodegen_Mask_AUTO, true)) {
    nh_log("failed to encode QR code for %s", utf8.constData());
    return QImage();
  }

  // quiet zone of 4 modules on each side, per the QR spec
  int size = qrcodegen_getSize(qrcode);
  int modules = size + 8;
  int scale = qMax(4, targetSize / modules);

  QImage image(modules * scale, modules * scale, QImage::Format_RGB32);
  image.fill(Qt::white);
  for (int y = 0; y < size; y++) {
    for (int x = 0; x < size; x++) {
      if (qrcodegen_getModule(qrcode, x, y)) {
        for (int dy = 0; dy < scale; dy++) {
          for (int dx = 0; dx < scale; dx++) {
            image.setPixel((x + 4) * scale + dx, (y + 4) * scale + dy, qRgb(0, 0, 0));
          }
        }
      }
    }
  }
  return image;
}
