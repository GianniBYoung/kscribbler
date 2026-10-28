#include <QFile>
#include <QFont>
#include <QPainter>
#include <QPixmap>
#include <QProcess>
#include <QWidgetAction>

#include <NickelHook.h>

#include "menucontroller.h"
#include "signindialog.h"

enum MenuOption { SIGN_IN, SYNC_NOW };

// drawIcon renders a bordered "K" so the plugin ships no image resources.
static QPixmap drawIcon(int height) {
  QPixmap pixmap(height, height);
  pixmap.fill(Qt::transparent);

  QPainter painter(&pixmap);
  painter.setRenderHint(QPainter::Antialiasing);
  QPen pen(Qt::black);
  pen.setWidth(qMax(2, height / 14));
  painter.setPen(pen);
  int inset = height / 6;
  painter.drawRoundedRect(QRect(inset, inset, height - 2 * inset, height - 2 * inset), height / 10, height / 10);

  QFont font("Georgia");
  font.setBold(true);
  font.setPixelSize(height / 2);
  painter.setFont(font);
  painter.drawText(pixmap.rect(), Qt::AlignCenter, "K");

  return pixmap;
}

MenuController::MenuController(int iconHeight, QWidget *parent) : QObject(parent) {
  icon = construct_TouchLabel(parent);
  TouchLabel__setHitStateEnabled(icon, false);
  icon->setPixmap(drawIcon(iconHeight));

  QObject::connect(icon, SIGNAL(tapped(bool)), this, SLOT(showMenu()));
}

void MenuController::addItem(NickelTouchMenu *menu, const QString &text, int option) {
  MenuTextItem *item = construct_MenuTextItem(menu);
  MenuTextItem__setText(item, text);
  MenuTextItem__registerForTapGestures(item);

  QWidgetAction *action = new QWidgetAction(menu);
  action->setDefaultWidget(item);
  menu->addAction(action);
  menu->addSeparator();

  QObject::connect(item, SIGNAL(tapped(bool)), action, SIGNAL(triggered()));
  QObject::connect(action, &QAction::triggered, menu, &QMenu::hide);
  QObject::connect(action, &QAction::triggered, this, [this, option]() { triggered(option); });
}

void MenuController::showMenu() {
  NickelTouchMenu *menu = construct_NickelTouchMenu(icon);
  NickelTouchMenu__showDecoration(menu, true);
  QObject::connect(menu, &QMenu::aboutToHide, menu, &QObject::deleteLater);

  bool signedIn = QFile::exists(Files::token);
  addItem(menu, signedIn ? "Sign in to Hardcover again" : "Sign in to Hardcover", SIGN_IN);
  addItem(menu, "Sync highlights now", SYNC_NOW);

  menu->ensurePolished();
  menu->popup(icon->parentWidget()->mapToGlobal(icon->geometry().bottomLeft()));
}

void MenuController::triggered(int option) {
  switch (option) {
  case SIGN_IN:
    new SignInDialog();
    break;
  case SYNC_NOW:
    QProcess::startDetached("/bin/sh", {"-c", QString("%1 >> %2 2>&1").arg(Files::cli, Files::log)});
    toast("Kscribbler sync started");
    break;
  }
}
