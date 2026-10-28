#include <QDir>
#include <QHBoxLayout>
#include <QString>

#include <NickelHook.h>

#include "kscribbler.h"
#include "menucontroller.h"

// Hook structure and Nickel symbols adapted from NickelHardcover (MIT, https://codeberg.org/StrayRose/NickelHardcover).

typedef QWidget ReadingMenuView;
static void (*ReadingMenuView__constructor)(ReadingMenuView *_this, QWidget *parent, bool unknown);
static void (*ReadingMenuView__constructor_2)(ReadingMenuView *_this, QWidget *parent, QByteArray const &unknownArray,
                                              bool unknownBool);

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
static struct nh_hook KscribblerHook[] = {
  { .sym = "_ZN15ReadingMenuViewC1EP7QWidgetb",               .sym_new = "_nh_ReadingMenuView__constructor",   .lib = "libnickel.so.1.0.0", .out = nh_symoutptr(ReadingMenuView__constructor),   .desc = "Inject kscribbler menu", .optional = true },
  { .sym = "_ZN15ReadingMenuViewC1EP7QWidgetRK10QByteArrayb", .sym_new = "_nh_ReadingMenuView__constructor_2", .lib = "libnickel.so.1.0.0", .out = nh_symoutptr(ReadingMenuView__constructor_2), .desc = "Inject kscribbler menu (4.44+)", .optional = true },
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

static bool kscribbler_uninstall() {
  nh_delete_file("/opt/bin/kscribbler");
  nh_delete_file("/mnt/onboard/.adds/nm/kscribbler");
  nh_delete_file("/mnt/onboard/.adds/nm/kscribbler-debug");
  return true;
}

NickelHook(.init = nullptr, .info = &Kscribbler, .hook = KscribblerHook, .dlsym = KscribblerDlsym,
           .uninstall = &kscribbler_uninstall);

static void injectMenuWidget(ReadingMenuView *parent) {
  QHBoxLayout *layout = parent->findChild<QHBoxLayout *>("bottomHorizontalLayout");
  QLabel *settingsIcon = parent->findChild<QLabel *>("settingsIcon");

  if (layout && settingsIcon) {
    MenuController *ctl = new MenuController(settingsIcon->height(), parent);
    layout->insertWidget(layout->count() - 1, ctl->icon);
  } else {
    nh_log("Error: unable to find bottomHorizontalLayout and settingsIcon");
  }
}

extern "C" __attribute__((visibility("default"))) void _nh_ReadingMenuView__constructor(ReadingMenuView *_this,
                                                                                        QWidget *parent, bool unknown) {
  ReadingMenuView__constructor(_this, parent, unknown);
  injectMenuWidget(parent);
}

extern "C" __attribute__((visibility("default"))) void
_nh_ReadingMenuView__constructor_2(ReadingMenuView *_this, QWidget *parent, QByteArray const &unknownArray,
                                   bool unknownBool) {
  ReadingMenuView__constructor_2(_this, parent, unknownArray, unknownBool);
  injectMenuWidget(parent);
}
