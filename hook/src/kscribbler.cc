#include <QCoreApplication>
#include <QString>

#include <NickelHook.h>

#include "kscribbler.h"
#include "menucontroller.h"

// Nickel symbols adapted from NickelHardcover (MIT, https://codeberg.org/StrayRose/NickelHardcover).

MainWindowController *(*MainWindowController__sharedInstance)();
QWidget *(*MainWindowController__pushView)(MainWindowController *mwc, QWidget *view);
void (*MainWindowController__toast)(MainWindowController *mwc, QString const &primary, QString const &secondary,
                                    int msecs);

void (*ConfirmationDialogFactory__showErrorDialog)(QString const &title, QString const &body);

N3Dialog *(*N3DialogFactory__getDialog)(QWidget *content, bool unknown);
void (*N3Dialog__setTitle)(N3Dialog *_this, QString const &title);

void (*TouchLabel__constructor)(TouchLabel *_this, QWidget *parent, QFlags<Qt::WindowType>);
void (*TouchLabel__setHitStateEnabled)(TouchLabel *_this, bool enabled);

TouchLabel *construct_TouchLabel(QWidget *parent) {
  TouchLabel *label = reinterpret_cast<TouchLabel *>(calloc(1, 128));
  TouchLabel__constructor(label, parent, 0);
  return label;
}

void (*N3ButtonLabel__constructor)(N3ButtonLabel *_this, QWidget *parent);

N3ButtonLabel *construct_N3ButtonLabel(QWidget *parent) {
  N3ButtonLabel *button = reinterpret_cast<N3ButtonLabel *>(calloc(1, 512));
  N3ButtonLabel__constructor(button, parent);
  return button;
}

void (*NickelTouchMenu__constructor)(NickelTouchMenu *_this, QWidget *parent, int pos);
void (*NickelTouchMenu__showDecoration)(NickelTouchMenu *_this, bool show);

NickelTouchMenu *construct_NickelTouchMenu(QWidget *parent) {
  NickelTouchMenu *menu = reinterpret_cast<NickelTouchMenu *>(calloc(1, 512));
  NickelTouchMenu__constructor(menu, parent, 0);
  return menu;
}

void (*MenuTextItem__constructor)(MenuTextItem *_this, QWidget *parent, bool checkable, bool italic);
void (*MenuTextItem__setText)(MenuTextItem *_this, QString const &text);
void (*MenuTextItem__registerForTapGestures)(MenuTextItem *_this);

MenuTextItem *construct_MenuTextItem(QWidget *parent) {
  MenuTextItem *item = reinterpret_cast<MenuTextItem *>(calloc(1, 256));
  MenuTextItem__constructor(item, parent, false, true);
  return item;
}

void toast(const QString &message) {
  MainWindowController *mwc = MainWindowController__sharedInstance();
  if (mwc && MainWindowController__toast) {
    MainWindowController__toast(mwc, message, QString(), 1500);
  }
}

static struct nh_info Kscribbler = (struct nh_info){
    .name = "kscribbler",
    .desc = "Sync Kobo highlights to Hardcover.app",
    .uninstall_flag = "/mnt/onboard/kscribbler_uninstall",
    .uninstall_xflag = nullptr,
};

// clang-format off
// no Nickel hooks: the menu icon is added by MenuInjector (see menucontroller.h)
static struct nh_hook KscribblerHook[] = {
  {0},
};

static struct nh_dlsym KscribblerDlsym[] = {
  { .name = "_ZN20MainWindowController14sharedInstanceEv",                   .out = nh_symoutptr(MainWindowController__sharedInstance) },
  { .name = "_ZN20MainWindowController8pushViewEP7QWidget",                  .out = nh_symoutptr(MainWindowController__pushView) },
  { .name = "_ZN20MainWindowController5toastERK7QStringS2_i",                .out = nh_symoutptr(MainWindowController__toast), .desc = "", .optional = true },
  { .name = "_ZN25ConfirmationDialogFactory15showErrorDialogERK7QStringS2_", .out = nh_symoutptr(ConfirmationDialogFactory__showErrorDialog) },
  { .name = "_ZN15N3DialogFactory9getDialogEP7QWidgetb",                     .out = nh_symoutptr(N3DialogFactory__getDialog) },
  { .name = "_ZN8N3Dialog8setTitleERK7QString",                              .out = nh_symoutptr(N3Dialog__setTitle) },
  { .name = "_ZN10TouchLabelC1EP7QWidget6QFlagsIN2Qt10WindowTypeEE",         .out = nh_symoutptr(TouchLabel__constructor) },
  { .name = "_ZN10TouchLabel18setHitStateEnabledEb",                         .out = nh_symoutptr(TouchLabel__setHitStateEnabled) },
  { .name = "_ZN13N3ButtonLabelC1EP7QWidget",                                .out = nh_symoutptr(N3ButtonLabel__constructor) },
  { .name = "_ZN15NickelTouchMenuC1EP7QWidget18DecorationPosition",          .out = nh_symoutptr(NickelTouchMenu__constructor) },
  { .name = "_ZN15NickelTouchMenu14showDecorationEb",                        .out = nh_symoutptr(NickelTouchMenu__showDecoration) },
  { .name = "_ZN12MenuTextItemC1EP7QWidgetbb",                               .out = nh_symoutptr(MenuTextItem__constructor) },
  { .name = "_ZN12MenuTextItem7setTextERK7QString",                          .out = nh_symoutptr(MenuTextItem__setText) },
  { .name = "_ZN12MenuTextItem22registerForTapGesturesEv",                   .out = nh_symoutptr(MenuTextItem__registerForTapGestures) },
  {0},
};
// clang-format on

static int kscribbler_init() {
  QCoreApplication *app = QCoreApplication::instance();
  if (!app) {
    nh_log("Error: no QCoreApplication yet, kscribbler menu disabled");
    return 0;
  }
  app->installEventFilter(new MenuInjector(app));
  nh_log("kscribbler: installed reading menu injector");
  return 0;
}

static bool kscribbler_uninstall() {
  nh_delete_file("/opt/bin/kscribbler");
  nh_delete_file("/mnt/onboard/.adds/nm/kscribbler");
  nh_delete_file("/mnt/onboard/.adds/nm/kscribbler-debug");
  return true;
}

NickelHook(.init = &kscribbler_init, .info = &Kscribbler, .hook = KscribblerHook, .dlsym = KscribblerDlsym,
           .uninstall = &kscribbler_uninstall);
