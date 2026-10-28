#pragma once

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
