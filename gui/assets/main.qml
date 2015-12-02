import QtQuick 2.5
import QtQuick.Controls 1.4
import QtQuick.Dialogs 1.2
import QtQuick.Layouts 1.2
import QtQuick.Window 2.2
import Qt.labs.settings 1.0

ApplicationWindow {
    id: applicationWindow
    visible: true
    width: 800
    height: 600
    title: "CBconvert"

    property int margin: 15
    property int screenWidth: Screen.width
    property int screenHeight: Screen.height

    function updateImage() {
        imagePreview.source = ""
        sizePreview.text = ""
        if(groupBoxPreview.checked) {
            if(c.len > 0) {
                imagePreview.source = "image://cover/" + c.get(listView.currentIndex).path
            }
        }
    }

    ColumnLayout {
        id: splitView1
        anchors.rightMargin: margin
        anchors.leftMargin: margin
        anchors.bottomMargin: margin
        anchors.topMargin: margin
        anchors.fill: parent

        RowLayout {
            id: rowLayout1
            anchors.bottom: rowLayout2.top
            anchors.right: parent.right
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.bottomMargin: 15
            Layout.minimumHeight: 250

            ScrollView {
                id: scrollview
                anchors.right: columnButtons.left
                anchors.bottom: parent.bottom
                anchors.left: parent.left
                anchors.top: parent.top
                anchors.rightMargin: margin
                frameVisible: true
                highlightOnFocus: false
                verticalScrollBarPolicy: 2
                flickableItem.interactive: true
                focus: true

                ListView {
                    id: listView
                    anchors.topMargin: 0
                    anchors.fill: parent
                    spacing: 1
                    focus: true

                    model: c.len

                    header: Rectangle {
                        height: 20
                        width: ListView.view.width
                        color: "#DCDCDC"

                        Text {
                            id: nameHeader
                            text: 'Name'
                            anchors.left: parent.left
                            anchors.leftMargin: 5
                            anchors.verticalCenter: parent.verticalCenter

                            MouseArea {
                                anchors.fill: parent
                                onClicked: {
                                    c.byName()
                                }
                            }
                        }

                        Text {
                            id: typeHeader
                            text: 'Type'
                            width: 40
                            anchors.right: sizeHeader.left
                            anchors.rightMargin: 40
                            anchors.verticalCenter: parent.verticalCenter
                            horizontalAlignment: Text.AlignHCenter

                            MouseArea {
                                anchors.fill: parent
                                onClicked: {
                                    c.byType()
                                }
                            }
                        }

                        Text {
                            id: sizeHeader
                            text: 'Size'
                            width: 40
                            anchors.right: parent.right
                            anchors.rightMargin: 15
                            anchors.verticalCenter: parent.verticalCenter
                            horizontalAlignment: Text.AlignHCenter

                            MouseArea {
                                anchors.fill: parent
                                onClicked: {
                                    c.bySize()
                                }
                            }
                        }
                    }

                    delegate: Item {
                        id: item1
                        width: ListView.view.width
                        height: 20

                        Text {
                            id: nameItem
                            text: c.get(index).name
                            anchors.left: parent.left
                            anchors.leftMargin: 5
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - 170
                            elide: Text.ElideRight
                        }

                        Text {
                            id: typeItem
                            width: 40
                            text: c.get(index).type
                            anchors.right: sizeItem.left
                            anchors.rightMargin: 40
                            anchors.verticalCenter: parent.verticalCenter
                            horizontalAlignment: Text.AlignHCenter
                        }

                        Text {
                            id: sizeItem
                            width: 40
                            text: c.get(index).sizeHuman
                            anchors.right: parent.right
                            anchors.rightMargin: 15
                            anchors.verticalCenter: parent.verticalCenter
                            horizontalAlignment: Text.AlignHCenter
                        }

                        MouseArea {
                            anchors.fill: parent
                            onClicked: {
                                listView.currentIndex = index
                                listView.forceActiveFocus()
                            }
                        }
                    }

                    highlight: Rectangle {
                        width: ListView.view ? ListView.view.width : undefined
                        color: "#326686"
                        opacity: 0.2
                        y: listView.currentItem.y
                        Behavior on y {
                            SpringAnimation {
                                spring: 3
                                damping: 0.2
                            }
                        }
                    }

                    onCurrentItemChanged: updateImage()
                }
            }

            ColumnLayout {
                id: columnButtons
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 0
                anchors.top: parent.top
                anchors.topMargin: 0
                anchors.right: parent.right
                anchors.rightMargin: 0

                ColumnLayout {
                    id: columnButtonsFiles
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    spacing: 5

                    Button {
                        id: buttonAddFile
                        objectName: "buttonAddFile"
                        text: "Add &Files"
                        onClicked: {
                            fileDialogFile.open()
                        }
                    }

                    Button {
                        id: buttonAddDir
                        objectName: "buttonAddDir"
                        text: "Add &Dir"
                        onClicked: {
                            fileDialogDir.open()
                        }
                    }

                    Button {
                        id: buttonRemove
                        objectName: "buttonRemove"
                        text: "Remove"
                        enabled: (c.len !== 0) ? true : false
                        onClicked: {
                            c.remove(listView.currentIndex)
                            updateImage()
                        }
                    }

                    Button {
                        id: buttonRemoveAll
                        objectName: "buttonRemoveAll"
                        text: "Remove All"
                        enabled: (c.len !== 0) ? true : false
                        onClicked: {
                            c.removeAll()
                            updateImage()
                        }
                    }
                }

                ColumnLayout {
                    id: columnButtonsActions
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: 0
                    spacing: 5

                    Button {
                        id: buttonThumbnail
                        objectName: "buttonThumbnail"
                        text: "Thumbnail"
                        anchors.bottom: buttonCover.top
                        anchors.bottomMargin: 5
                        tooltip: "Extract Thumbnail (freedesktop spec.)"
                        enabled: (textFieldOutDir.text != "" && c.len !== 0) ? true : false
                    }

                    Button {
                        id: buttonCover
                        objectName: "buttonCover"
                        text: "Cover"
                        anchors.bottom: buttonConvert.top
                        anchors.bottomMargin: 15
                        tooltip: "Extract Cover"
                        enabled: (textFieldOutDir.text != "" && c.len !== 0) ? true : false
                    }

                    Button {
                        id: buttonConvert
                        objectName: "buttonConvert"
                        text: "&Convert"
                        tooltip: "Convert archives and documents"
                        enabled: (textFieldOutDir.text != "" && c.len !== 0) ? true : false
                    }
                }
            }
        }

        RowLayout {
            id: rowLayout2
            spacing: 0
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.left: parent.left

            ColumnLayout {
                id: columnLeft
                anchors.right: columnMiddle.left
                anchors.rightMargin: margin
                anchors.left: parent.left
                anchors.leftMargin: 0
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 0
                anchors.top: parent.top
                anchors.topMargin: 0
                Layout.minimumWidth: 200

                GroupBox {
                    id: groupBoxPreview
                    checkable: true
                    flat: true
                    anchors.fill: parent
                    title: "Preview"

                    Image {
                        id: imagePreview
                        anchors.fill: parent
                        fillMode: Image.PreserveAspectFit
                        asynchronous: true
                        cache: false
                    }

                    BusyIndicator {
                        running: imagePreview.status === Image.Loading
                        anchors.verticalCenter: parent.verticalCenter
                        anchors.horizontalCenter: parent.horizontalCenter
                    }

                    onCheckedChanged: updateImage()
                }

                Text {
                    id: sizePreview
                    objectName: "sizePreview"
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.top: groupBoxPreview.bottom
                    anchors.topMargin: -5
                }
            }

            ColumnLayout {
                id: columnMiddle
                anchors.right: columnRight.left
                anchors.rightMargin: margin
                anchors.top: parent.top
                anchors.topMargin: 0
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 0
                Layout.fillHeight: true

                GroupBox {
                    id: groupBoxInput
                    flat: true
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    title: "Input"
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.left: parent.left
                    anchors.leftMargin: 0

                    ColumnLayout {
                        id: columnInput
                        anchors.bottomMargin: 0
                        anchors.fill: parent
                        spacing: 5

                        CheckBox {
                            id: checkBoxRecursive
                            objectName: "checkBoxRecursive"
                            text: "Recurse SubDirectories"
                        }

                        CheckBox {
                            id: checkBoxNoRGB
                            objectName: "checkBoxNoRGB"
                            text: "Only Grayscaled Images"
                        }

                        CheckBox {
                            id: checkBoxConvertCover
                            objectName: "checkBoxConvertCover"
                            text: "Exclude Cover"
                        }

                        RowLayout {
                            anchors.bottom: parent.bottom
                            anchors.bottomMargin: 0
                            anchors.left: parent.left
                            anchors.leftMargin: 0
                            anchors.right: parent.right
                            anchors.rightMargin: 0
                            spacing: 5

                            SpinBox {
                                id: spinboxSize
                                objectName: "spinboxSize"
                                stepSize: 10
                                prefix: ""
                                maximumValue: 1000
                                suffix: " MiB"
                            }

                            Text {
                                text: "Minimum Size"
                            }
                        }
                    }
                }

                GroupBox {
                    id: groupBoxTransform
                    flat: true
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: 0
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    title: "Transform"

                    ColumnLayout {
                        id: columnTransform
                        anchors.fill: parent
                        spacing: 5

                        Button {
                            id: buttonLevels
                            text: "Levels..."
                            enabled: (c.len > 0) ? true : false
                            onClicked: {
                                levelsDialog.open()
                            }
                        }

                        Text {
                            text: "Brightness: " + sliderBrightness.value
                        }

                        Slider {
                            id: sliderBrightness
                            objectName: "sliderBrightness"
                            value: 0
                            stepSize: 1
                            minimumValue: -100
                            maximumValue: 100
                            activeFocusOnPress: true
                            updateValueWhileDragging: false
                            enabled: (c.len > 0) ? true : false
                            onValueChanged: updateImage()
                        }

                        Text {
                            text: "Contrast: " + sliderContrast.value
                        }

                        Slider {
                            id: sliderContrast
                            objectName: "sliderContrast"
                            value: 0
                            maximumValue: 100
                            stepSize: 1
                            minimumValue: -100
                            activeFocusOnPress: true
                            updateValueWhileDragging: false
                            enabled: (c.len > 0) ? true : false
                            onValueChanged: updateImage()
                        }

                        Text {
                            text: "Flip:"
                        }

                        ComboBox {
                            id: comboBoxFlip
                            objectName: "comboBoxFlip"
                            enabled: (c.len > 0) ? true : false

                            model: ListModel {
                                ListElement {
                                    text: "None"
                                }

                                ListElement {
                                    text: "Horizontal"
                                }

                                ListElement {
                                    text: "Vertical"
                                }
                            }

                            onActivated: updateImage()
                        }

                        Text {
                            text: "Rotate:"
                        }

                        ComboBox {
                            id: comboBoxRotate
                            objectName: "comboBoxRotate"
                            enabled: (c.len > 0) ? true : false

                            model: ListModel {
                                ListElement {
                                    text: "0"
                                }

                                ListElement {
                                    text: "90"
                                }

                                ListElement {
                                    text: "180"
                                }

                                ListElement {
                                    text: "270"
                                }
                            }

                            onActivated: updateImage()
                        }
                    }
                }
            }

            ColumnLayout {
                id: columnRight
                anchors.top: parent.top
                anchors.topMargin: 0
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 0
                anchors.right: parent.right
                anchors.rightMargin: 0
                Layout.fillHeight: true

                GroupBox {
                    id: groupBoxOutput
                    flat: true
                    title: "Output"
                    anchors.top: parent.top
                    anchors.topMargin: 0
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    anchors.right: parent.right
                    anchors.rightMargin: 0

                    ColumnLayout {
                        id: columnOutput
                        anchors.bottomMargin: 0
                        anchors.fill: parent
                        spacing: 5

                        RowLayout {
                            id: rowLayoutOutput
                            anchors.top: parent.top
                            anchors.topMargin: 0
                            anchors.left: parent.left
                            anchors.leftMargin: 0
                            anchors.right: parent.right
                            anchors.rightMargin: 0
                            spacing: 5

                            TextField {
                                id: textFieldOutDir
                                objectName: "textFieldOutDir"
                                anchors.right: buttonBrowse.left
                                anchors.rightMargin: 5
                                anchors.left: parent.left
                                anchors.leftMargin: 0
                                placeholderText: "Output Directory"

                                Settings {
                                    id: settingsOutDir
                                    property alias text: textFieldOutDir.text
                                }
                            }

                            Button {
                                id: buttonBrowse
                                text: "..."
                                anchors.right: parent.right
                                anchors.rightMargin: 0
                                anchors.verticalCenter: parent.verticalCenter
                                onClicked: {
                                    fileDialogOutput.open()
                                }
                            }
                        }

                        TextField {
                            id: textFieldSuffix
                            objectName: "textFieldSuffix"
                            anchors.left: parent.left
                            anchors.leftMargin: 0
                            anchors.right: parent.right
                            anchors.rightMargin: 0
                            placeholderText: "Add Suffix to Output File"
                        }

                        CheckBox {
                            id: checkBoxNonImage
                            objectName: "checkBoxNonImage"
                            text: "Remove Non-Image Files"
                        }
                    }
                }


                GroupBox {
                    id: groupBoxImage
                    flat: true
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    title: "Image"
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: 0
                    anchors.right: parent.right
                    anchors.rightMargin: 0

                    ColumnLayout {
                        id: columnFormat
                        anchors.fill: parent
                        spacing: 5

                        Text {
                            text: "Format:"
                        }

                        ComboBox {
                            id: comboBoxFormat
                            objectName: "comboBoxFormat"
                            enabled: (c.len > 0) ? true : false

                            model: ListModel {
                                ListElement {
                                    text: "JPEG"
                                }

                                ListElement {
                                    text: "PNG"
                                }

                                ListElement {
                                    text: "GIF"
                                }

                                ListElement {
                                    text: "BMP"
                                }

                                ListElement {
                                    text: "TIFF"
                                }
                            }

                            onActivated: updateImage()
                        }

                        Text {
                            text: "Size:"
                        }

                        RowLayout {
                            spacing: 5

                            TextField {
                                id: width
                                objectName: "width"
                                placeholderText: "width"
                                maximumLength: 4
                                implicitWidth: 50
                                onAccepted: updateImage()
                            }

                            Text {
                                id: x
                                text: "x"
                                anchors.verticalCenter: parent.verticalCenter
                                anchors.horizontalCenter: parent.horizontalCenter
                            }

                            TextField {
                                id: height
                                objectName: "height"
                                placeholderText: "height"
                                maximumLength: 4
                                implicitWidth: 50
                                onAccepted: updateImage()
                            }
                        }

                        CheckBox {
                            id: checkBoxFit
                            objectName: "checkBoxFit"
                            text: "Best Fit"
                            enabled: (c.len > 0) ? true : false
                            onClicked: updateImage()
                        }

                        Text {
                            text: "Resize Algorithm:"
                        }

                        ComboBox {
                            id: comboBoxFilter
                            objectName: "comboBoxFilter"
                            currentIndex: 2
                            enabled: (c.len > 0) ? true : false

                            model: ListModel {
                                ListElement {
                                    text: "NearestNeighbor"
                                }

                                ListElement {
                                    text: "Box"
                                }

                                ListElement {
                                    text: "Linear"
                                }

                                ListElement {
                                    text: "MitchellNetravali"
                                }

                                ListElement {
                                    text: "CatmullRom"
                                }

                                ListElement {
                                    text: "Gaussian"
                                }

                                ListElement {
                                    text: "Lanczos"
                                }
                            }

                            onActivated: updateImage()

                            Settings {
                                id: settingsFilter
                                property alias currentIndex: comboBoxFilter.currentIndex
                            }
                        }

                        Text {
                            text: "Quality: " + sliderQuality.value
                        }

                        Slider {
                            id: sliderQuality
                            objectName: "sliderQuality"
                            stepSize: 1
                            value: 75
                            maximumValue: 100
                            activeFocusOnPress: true
                            updateValueWhileDragging: false
                            enabled: (c.len > 0 && comboBoxFormat.currentText == "JPEG") ? true : false
                            onValueChanged: updateImage()
                        }

                        CheckBox {
                            id: checkBoxGrayscale
                            objectName: "checkBoxGrayscale"
                            text: "Convert to Grayscale"
                            enabled: (c.len > 0) ? true : false
                            onClicked: updateImage()
                        }

                        Settings {
                            id: settingsQuality
                            property alias value: sliderQuality.value
                        }
                    }
                }
            }
        }
    }

    FileDialog {
        id: fileDialogFile
        modality: Qt.WindowModal
        title: "Add Files"
        selectFolder: false
        selectMultiple: true
        selectExisting: true
        sidebarVisible: true
        nameFilters: [ "Comic files (*.rar *.zip *.7z *.gz *.bz2 *.cbr *.cbz *.cb7 *.cbt *.pdf *.epub *.xps)" ]
        onAccepted: {
            c.addUrls(decodeURIComponent(fileUrls.join("_CBSEP_")))
        }
    }

    FileDialog {
        id: fileDialogDir
        modality: Qt.WindowModal
        title: "Add Directory"
        selectFolder: true
        sidebarVisible: true
        onAccepted: {
            c.addUrls(decodeURIComponent(fileUrl.toString()))
        }
    }

    FileDialog {
        id: fileDialogOutput
        modality: Qt.WindowModal
        title: "Output Directory"
        selectFolder: true
        sidebarVisible: true
        onAccepted: {
            textFieldOutDir.text = decodeURIComponent(fileUrl.toString().replace("file://", ""))
        }
    }

    Dialog {
        id: levelsDialog
        objectName: "levelsDialog"
        title: "Levels"
        standardButtons: StandardButton.Close
        width: 230
        height: 150

        ColumnLayout {
            anchors.fill: parent

            Text {
                text: "Input Levels:"
            }

            RowLayout {
                anchors.left: parent.left
                anchors.leftMargin: 0
                anchors.right: parent.right
                anchors.rightMargin: 0
                spacing: 5

                SpinBox {
                    id: spinboxLevelsInMin
                    objectName: "spinboxLevelsInMin"
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    stepSize: 1
                    maximumValue: 255
                    value: 0
                    onEditingFinished: updateImage()
                    Keys.onReturnPressed: {
                        event.accepted = true
                    }
                }

                SpinBox {
                    id: spinboxLevelsGamma
                    objectName: "spinboxLevelsGamma"
                    anchors.horizontalCenter: parent.horizontalCenter
                    decimals: 2
                    stepSize: 0.01
                    maximumValue: 10.00
                    value: 1.00
                    onEditingFinished: updateImage()
                }

                SpinBox {
                    id: spinboxLevelsInMax
                    objectName: "spinboxLevelsInMax"
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    stepSize: 1
                    maximumValue: 255
                    value: 255
                    onEditingFinished: updateImage()
                }
            }

            Text {
                text: "Output Levels:"
            }

            RowLayout {
                anchors.left: parent.left
                anchors.leftMargin: 0
                anchors.right: parent.right
                anchors.rightMargin: 0
                spacing: 5

                SpinBox {
                    id: spinboxLevelsOutMin
                    objectName: "spinboxLevelsOutMin"
                    anchors.left: parent.left
                    anchors.leftMargin: 0
                    stepSize: 1
                    maximumValue: 255
                    value: 0
                    onEditingFinished: updateImage()
                }

                SpinBox {
                    id: spinboxLevelsOutMax
                    objectName: "spinboxLevelsOutMax"
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    stepSize: 1
                    maximumValue: 255
                    value: 255
                    onEditingFinished: updateImage()
                }
            }
        }
    }

    statusBar: StatusBar {

        RowLayout {
            anchors.fill: parent

            Label {
                id: labelStatus
                objectName: "labelStatus"
                text: "Ready"
                anchors.verticalCenter: parent.verticalCenter
            }

            Rectangle {
                id: rectangle1
                Layout.fillWidth: true
                anchors.verticalCenter: parent.verticalCenter

                Label {
                    id: labelPercent
                    objectName: "labelPercent"
                    font.pointSize: 9
                    anchors.right: progressBar.left
                    anchors.rightMargin: 5
                    anchors.verticalCenter: parent.verticalCenter
                }

                ProgressBar {
                    id: progressBar
                    objectName: "progressBar"
                    visible: false
                    value: 0.0
                    minimumValue : 0.0
                    maximumValue : 100.0
                    anchors.bottom: parent.bottom
                    anchors.bottomMargin: -10
                    anchors.top: parent.top
                    anchors.topMargin: -10
                    anchors.right: labelProgress.left
                    anchors.rightMargin: 5
                }

                Label {
                    id: labelProgress
                    objectName: "labelProgress"
                    font.pointSize: 9
                    anchors.right: parent.right
                    anchors.rightMargin: 0
                    anchors.verticalCenter: parent.verticalCenter
                }
            }
        }
    }

}
