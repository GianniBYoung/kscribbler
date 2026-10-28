#pragma once

#include <QEvent>
#include <QObject>

#include "kscribbler.h"

// Reading-menu icon that opens the kscribbler menu.
class MenuController : public QObject {
  Q_OBJECT

public:
  MenuController(int iconHeight, QWidget *parent);
  TouchLabel *icon = nullptr;

public Q_SLOTS:
  void showMenu();

private:
  void addItem(NickelTouchMenu *menu, const QString &text, int option);
  void triggered(int option);
};

// Adds the menu icon when Nickel shows the reading menu. Uses an app-wide event filter instead of hooking
// ReadingMenuView, because NickelHook keeps only the last plugin's hook per symbol (that would hide NickelHardcover).
class MenuInjector : public QObject {
  Q_OBJECT

public:
  using QObject::QObject;

protected:
  bool eventFilter(QObject *watched, QEvent *event) override;
};
