#pragma once

#include <QFrame>
#include <QImage>
#include <QJsonObject>
#include <QVBoxLayout>

#include "kscribbler.h"

// Native dialog showing the Hardcover device-login QR code, user code and a Continue button.
class SignInDialog : public QFrame {
  Q_OBJECT

public:
  SignInDialog();

private Q_SLOTS:
  void continueTapped();

private:
  void requestFinished(const QJsonObject &result);
  void setFinished(const QJsonObject &result);
  void fail(const QString &message);
  static QImage buildQrCode(const QString &text, int targetSize);

  N3Dialog *dialog = nullptr;
  QVBoxLayout *layout = nullptr;
  QLabel *status = nullptr;
  QString deviceCode;
  bool busy = false;
};
