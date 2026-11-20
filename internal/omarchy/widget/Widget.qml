import QtQuick
import Quickshell
import qs.Commons
import qs.Ui

// eipgam's icon in Omarchy's bar, put there by eipgam (Settings → Bar icon).
// A click drops down its panel under the bar, where the click was; a right
// click opens its window. eipgam is started if it isn't running.
BarWidget {
  id: root
  moduleName: "com.sichengchen.eipgam"

  // written in by eipgam when it puts the widget here
  readonly property string eipgam: "__MAGPIE__"

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: String.fromCodePoint(0xf15c6) // nf-md-bird
    tooltipText: "eipgam"
    onPressed: function(b) {
      Quickshell.execDetached(b === Qt.RightButton ? [root.eipgam, "app"] : [root.eipgam, "panel"])
    }
  }
}
