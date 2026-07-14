ObjC.import("AppKit");
ObjC.import("Foundation");

function pasteboard() {
  return $.NSPasteboard.generalPasteboard;
}

function writePasteboardItems(items) {
  const board = pasteboard();
  board.clearContents;
  if (!board.writeObjects(items)) {
    throw new Error("无法写入测试剪贴板");
  }
}

function snapshot(path) {
  const serialized = [];
  const items = pasteboard().pasteboardItems;
  if (items) {
    for (let itemIndex = 0; itemIndex < Number(items.count); itemIndex += 1) {
      const item = items.objectAtIndex(itemIndex);
      const values = [];
      const types = item.types;
      for (let typeIndex = 0; typeIndex < Number(types.count); typeIndex += 1) {
        const type = types.objectAtIndex(typeIndex);
        const data = item.dataForType(type);
        if (data) {
          values.push({
            type: ObjC.unwrap(type),
            data: ObjC.unwrap(data.base64EncodedStringWithOptions(0))
          });
        }
      }
      serialized.push(values);
    }
  }
  const content = $(JSON.stringify(serialized));
  if (!content.writeToFileAtomicallyEncodingError($(path), true, $.NSUTF8StringEncoding, $())) {
    throw new Error("无法保存剪贴板快照");
  }
}

function restore(path) {
  const content = $.NSString.stringWithContentsOfFileEncodingError(
    $(path),
    $.NSUTF8StringEncoding,
    $()
  );
  if (!content) {
    throw new Error("无法读取剪贴板快照");
  }
  const serialized = JSON.parse(ObjC.unwrap(content));
  const items = $.NSMutableArray.array;
  serialized.forEach(function (values) {
    const item = $.NSPasteboardItem.alloc.init;
    values.forEach(function (value) {
      const data = $.NSData.alloc.initWithBase64EncodedStringOptions($(value.data), 0);
      if (!data || !item.setDataForType(data, $(value.type))) {
        throw new Error("无法恢复剪贴板数据类型：" + value.type);
      }
    });
    items.addObject(item);
  });
  const board = pasteboard();
  board.clearContents;
  if (Number(items.count) > 0 && !board.writeObjects(items)) {
    throw new Error("无法恢复剪贴板");
  }
}

function setImage(path, type) {
  const data = $.NSData.dataWithContentsOfFile($(path));
  if (!data) {
    throw new Error("无法读取图片 fixture");
  }
  let pasteboardData = data;
  if (type === "public.tiff") {
    const image = $.NSImage.alloc.initWithData(data);
    pasteboardData = image ? image.TIFFRepresentation : null;
  }
  if (!pasteboardData) {
    throw new Error("无法生成 TIFF fixture");
  }
  const item = $.NSPasteboardItem.alloc.init;
  if (!item.setDataForType(pasteboardData, $(type))) {
    throw new Error("无法设置图片类型：" + type);
  }
  writePasteboardItems($.NSArray.arrayWithObject(item));
}

function setFiles(rawPaths) {
  const paths = JSON.parse(rawPaths);
  const urls = $.NSMutableArray.array;
  paths.forEach(function (path) {
    urls.addObject($.NSURL.fileURLWithPath($(path)));
  });
  writePasteboardItems(urls);
}

function setText(value) {
  const item = $.NSPasteboardItem.alloc.init;
  if (!item.setStringForType($(value), $.NSPasteboardTypeString)) {
    throw new Error("无法设置文本剪贴板");
  }
  writePasteboardItems($.NSArray.arrayWithObject(item));
}

function run(argv) {
  const command = argv[0];
  if (command === "snapshot") {
    snapshot(argv[1]);
    return "";
  }
  if (command === "restore") {
    restore(argv[1]);
    return "";
  }
  if (command === "set-png") {
    setImage(argv[1], "public.png");
    return "";
  }
  if (command === "set-tiff") {
    setImage(argv[1], "public.tiff");
    return "";
  }
  if (command === "set-files") {
    setFiles(argv[1]);
    return "";
  }
  if (command === "set-text") {
    setText(argv[1]);
    return "";
  }
  throw new Error("unknown command: " + command);
}
