#include <QJsonDocument>

#include <NickelHook.h>

#include "cli.h"
#include "kscribbler.h"

CLI *CLI::oauthRequest() { return new CLI({"--oauth-request"}); }

CLI *CLI::oauthSet(const QString &deviceCode) { return new CLI({"--oauth-set=" + deviceCode}); }

CLI::CLI(const QStringList &arguments) : QObject(), process(new QProcess(this)) {
  process->setStandardErrorFile(Files::log, QIODevice::Append);

  QObject::connect(process, SIGNAL(finished(int, QProcess::ExitStatus)), this,
                   SLOT(processFinished(int, QProcess::ExitStatus)));
  QObject::connect(process, SIGNAL(error(QProcess::ProcessError)), this, SLOT(processError(QProcess::ProcessError)));

  process->start(Files::cli, arguments, QIODevice::ReadOnly);
}

void CLI::emitError(const QString &message) {
  if (done) {
    return;
  }
  done = true;

  QJsonObject result;
  result.insert("status", QString("error"));
  result.insert("message", message);
  Q_EMIT finished(result);
  deleteLater();
}

void CLI::processFinished(int exitCode, QProcess::ExitStatus status) {
  if (done) {
    return;
  }

  QList<QByteArray> lines = process->readAllStandardOutput().trimmed().split('\n');
  QJsonDocument doc = QJsonDocument::fromJson(lines.last().trimmed());

  if (!doc.isObject()) {
    emitError(status == QProcess::CrashExit ? QString("kscribbler crashed")
                                            : QString("kscribbler failed (exit %1). See kscribbler.log").arg(exitCode));
    return;
  }

  done = true;
  Q_EMIT finished(doc.object());
  deleteLater();
}

void CLI::processError(QProcess::ProcessError error) {
  if (error == QProcess::FailedToStart) {
    nh_log("failed to start %s", Files::cli);
    emitError(QString("Could not start %1").arg(Files::cli));
  }
}
