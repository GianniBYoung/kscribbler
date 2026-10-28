#pragma once

#include <QDialog>
#include <QLabel>
#include <QMenu>
#include <QWidget>

namespace Files {
constexpr char const *cli = "/opt/bin/kscribbler";
constexpr char const *log = "/mnt/onboard/.adds/kscribbler/kscribbler.log";
constexpr char const *token = "/mnt/onboard/.adds/kscribbler/token.json";
constexpr char const *adds_directory = "/mnt/onboard/.adds/kscribbler";
} // namespace Files

// Nickel symbols resolved by NickelHook (see kscribbler.cc). Signatures follow NickelHardcover/NickelMenu.
typedef QObject MainWindowController;
extern MainWindowController *(*MainWindowController__sharedInstance)();
extern QWidget *(*MainWindowController__pushView)(MainWindowController *mwc, QWidget *view);
extern void (*MainWindowController__toast)(MainWindowController *mwc, QString const &primary,
                                           QString const &secondary, int msecs);

extern void (*ConfirmationDialogFactory__showErrorDialog)(QString const &title, QString const &body);

typedef QDialog N3Dialog;
extern N3Dialog *(*N3DialogFactory__getDialog)(QWidget *content, bool unknown);
extern void (*N3Dialog__setTitle)(N3Dialog *_this, QString const &title);

typedef QLabel TouchLabel;
TouchLabel *construct_TouchLabel(QWidget *parent);
extern void (*TouchLabel__setHitStateEnabled)(TouchLabel *_this, bool enabled);

typedef TouchLabel N3ButtonLabel;
N3ButtonLabel *construct_N3ButtonLabel(QWidget *parent);

typedef QMenu NickelTouchMenu;
NickelTouchMenu *construct_NickelTouchMenu(QWidget *parent);
extern void (*NickelTouchMenu__showDecoration)(NickelTouchMenu *_this, bool show);

typedef QWidget MenuTextItem;
MenuTextItem *construct_MenuTextItem(QWidget *parent);
extern void (*MenuTextItem__setText)(MenuTextItem *_this, QString const &text);
extern void (*MenuTextItem__registerForTapGestures)(MenuTextItem *_this);

void toast(const QString &message);
