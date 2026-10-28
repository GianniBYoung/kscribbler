#pragma once

#include <QJsonObject>
#include <QObject>
#include <QProcess>
#include <QStringList>

// Runs the kscribbler binary and emits the JSON line it prints on stdout.
class CLI : public QObject {
  Q_OBJECT

public:
  static CLI *oauthRequest();
  static CLI *oauthSet(const QString &deviceCode);

Q_SIGNALS:
  void finished(const QJsonObject &result);

private Q_SLOTS:
  void processFinished(int exitCode, QProcess::ExitStatus status);
  void processError(QProcess::ProcessError error);

private:
  explicit CLI(const QStringList &arguments);
  void emitError(const QString &message);

  QProcess *process;
  bool done = false;
};
