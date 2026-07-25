ObjC.import("AppKit");
ObjC.import("Foundation");

const PRESERVED_TYPES = [
  { types: ["public.png"], extension: "png" },
  { types: ["public.jpeg", "public.jpg"], extension: "jpg" },
  { types: ["com.compuserve.gif", "public.gif"], extension: "gif" },
  { types: ["org.webmproject.webp", "public.webp"], extension: "webp" }
];

function imagePolicy(types) {
  for (let index = 0; index < PRESERVED_TYPES.length; index += 1) {
    const candidate = PRESERVED_TYPES[index];
    for (let typeIndex = 0; typeIndex < candidate.types.length; typeIndex += 1) {
      if (types.indexOf(candidate.types[typeIndex]) !== -1) {
        return {
          type: candidate.types[typeIndex],
          extension: candidate.extension,
          preserve: true
        };
      }
    }
  }

  return { type: "", extension: "png", preserve: false };
}

function clipboardTypes(pasteboard) {
  const result = [];
  const types = pasteboard.types;
  for (let index = 0; index < Number(types.count); index += 1) {
    result.push(ObjC.unwrap(types.objectAtIndex(index)));
  }
  return result;
}

function finderFilePaths(pasteboard) {
  const classes = $.NSMutableArray.array;
  classes.addObject($.NSURL);
  classes.addObject($.NSPasteboardItem);
  const options = $.NSDictionary.dictionaryWithObjectForKey(
    $.NSNumber.numberWithBool(true),
    $.NSPasteboardURLReadingFileURLsOnlyKey
  );
  const objects = pasteboard.readObjectsForClassesOptions(classes, options);
  const paths = [];
  if (!objects) {
    return paths;
  }
  for (let index = 0; index < Number(objects.count); index += 1) {
    const object = objects.objectAtIndex(index);
    if (object.isKindOfClass($.NSURL)) {
      paths.push(ObjC.unwrap(object.path));
      continue;
    }
    const rawUrl = object.stringForType($.NSPasteboardTypeFileURL);
    const url = rawUrl ? $.NSURL.URLWithString(rawUrl) : null;
    if (url && url.isFileURL) {
      paths.push(ObjC.unwrap(url.path));
    }
  }
  return paths;
}

function assertRegularFile(path) {
  if (/[\x00-\x1F]/.test(path)) {
    throw new Error("暂不支持文件名中包含控制字符的文件");
  }

  const attributes = $.NSFileManager.defaultManager.attributesOfItemAtPathError($(path), $());
  if (!attributes) {
    throw new Error("Finder 文件不存在或无法读取：" + path);
  }
  const fileType = ObjC.unwrap(attributes.objectForKey($.NSFileType));
  if (fileType !== ObjC.unwrap($.NSFileTypeRegular)) {
    throw new Error("只支持单个普通文件，不支持目录或特殊文件");
  }
}

function finderFileResult(filePaths) {
  if (filePaths.length > 1) {
    throw new Error("一次只能上传一个 Finder 文件");
  }
  if (filePaths.length === 0) {
    return "";
  }
  assertRegularFile(filePaths[0]);
  return "file\u001f" + filePaths[0] + "\u001f\u001f0";
}

function writePreservedImage(pasteboard, policy, outputBase) {
  const data = pasteboard.dataForType($(policy.type));
  if (!data) {
    throw new Error("无法读取剪贴板图片数据");
  }
  const outputPath = outputBase + "." + policy.extension;
  if (!data.writeToFileAtomically($(outputPath), true)) {
    throw new Error("无法写入临时图片");
  }
  return outputPath;
}

function writeConvertedPng(pasteboard, outputBase) {
  const image = $.NSImage.alloc.initWithPasteboard(pasteboard);
  if (!image) {
    throw new Error("剪贴板中没有可上传的单个文件或图片");
  }
  const tiffData = image.TIFFRepresentation;
  const bitmap = tiffData ? $.NSBitmapImageRep.imageRepWithData(tiffData) : null;
  const pngData = bitmap
    ? bitmap.representationUsingTypeProperties(
        $.NSBitmapImageFileTypePNG,
        $.NSDictionary.dictionary
      )
    : null;
  if (!pngData) {
    throw new Error("无法把剪贴板图片转换为 PNG");
  }
  const outputPath = outputBase + ".png";
  if (!pngData.writeToFileAtomically($(outputPath), true)) {
    throw new Error("无法写入临时 PNG 图片");
  }
  return outputPath;
}

function readClipboard(outputBase) {
  if (!outputBase || /[\r\n\t\0]/.test(outputBase)) {
    throw new Error("临时图片路径无效");
  }

  const pasteboard = $.NSPasteboard.generalPasteboard;
  const filePaths = finderFilePaths(pasteboard);
  const fileResult = finderFileResult(filePaths);
  if (fileResult) {
    return fileResult;
  }

  const policy = imagePolicy(clipboardTypes(pasteboard));
  const outputPath = policy.preserve
    ? writePreservedImage(pasteboard, policy, outputBase)
    : writeConvertedPng(pasteboard, outputBase);
  return "image\u001f" + outputPath + "\u001f" + policy.extension + "\u001f1";
}

function run(argv) {
  const command = argv.length > 0 ? argv[0] : "read";
  if (command === "classify-types") {
    const types = JSON.parse(argv[1] || "[]");
    if (!Array.isArray(types)) {
      throw new Error("types 必须是数组");
    }
    return JSON.stringify(imagePolicy(types));
  }
  if (command === "read") {
    return readClipboard(argv[1]);
  }
  if (command === "validate-file-paths") {
    const paths = JSON.parse(argv[1] || "[]");
    if (!Array.isArray(paths) || paths.length === 0) {
      throw new Error("paths 必须是非空数组");
    }
    return finderFileResult(paths);
  }
  throw new Error("unknown command: " + command);
}
