ObjC.import("Foundation");

function environment(name) {
  const value = $.NSProcessInfo.processInfo.environment.objectForKey(name);
  return value ? ObjC.unwrap(value) : "";
}

function statePath() {
  const dataDir = environment("alfred_workflow_data");
  return dataDir ? dataDir + "/recent_hosts.json" : "";
}

function readRecentHosts() {
  const path = statePath();
  if (!path) {
    return [];
  }

  try {
    const content = $.NSString.stringWithContentsOfFileEncodingError(
      $(path),
      $.NSUTF8StringEncoding,
      $()
    );
    if (!content) {
      return [];
    }
    const parsed = JSON.parse(ObjC.unwrap(content));
    if (!parsed || parsed.version !== 1 || !Array.isArray(parsed.recent)) {
      return [];
    }
    return parsed.recent.filter(function (item) {
      return item && typeof item.host === "string" && typeof item.remote_dir === "string";
    });
  } catch (_) {
    return [];
  }
}

function writeRecentHosts(recent) {
  const dataDir = environment("alfred_workflow_data");
  if (!dataDir) {
    throw new Error("Alfred workflow data 目录不可用");
  }

  const fileManager = $.NSFileManager.defaultManager;
  const created = fileManager.createDirectoryAtPathWithIntermediateDirectoriesAttributesError(
    $(dataDir),
    true,
    $(),
    $()
  );
  if (!created) {
    throw new Error("无法创建 Alfred workflow data 目录");
  }

  const content = $(JSON.stringify({ version: 1, recent: recent.slice(0, 50) }));
  const written = content.writeToFileAtomicallyEncodingError(
    $(statePath()),
    true,
    $.NSUTF8StringEncoding,
    $()
  );
  if (!written) {
    throw new Error("无法写入最近使用主机状态");
  }
}

function validateConfig() {
  const rawConfig = environment("hosts_json");
  let hosts;
  try {
    hosts = JSON.parse(rawConfig);
  } catch (_) {
    throw new Error("hosts_json 不是合法 JSON");
  }

  if (!Array.isArray(hosts) || hosts.length === 0) {
    throw new Error("hosts_json 至少配置一台主机");
  }

  const names = Object.create(null);
  const targets = Object.create(null);
  const normalized = hosts.map(function (item, index) {
    if (!item || typeof item !== "object" || Array.isArray(item)) {
      throw new Error("第 " + (index + 1) + " 项必须是对象");
    }

    ["name", "host", "remote_dir"].forEach(function (field) {
      if (typeof item[field] !== "string" || item[field].trim() === "") {
        throw new Error("第 " + (index + 1) + " 项缺少必填字段 " + field);
      }
    });

    const name = item.name.trim();
    const host = item.host.trim();
    const remoteDir = item.remote_dir.trim();
    if (names[name]) {
      throw new Error("name 不能重复：" + name);
    }
    if (!/^[A-Za-z0-9][A-Za-z0-9._-]*$/.test(host)) {
      throw new Error("SSH Host 只能包含字母、数字、点、下划线和连字符");
    }
    if (remoteDir.charAt(0) !== "/" || /[\x00-\x1f]/.test(remoteDir)) {
      throw new Error("remote_dir 必须是无控制字符的绝对路径");
    }

    const targetKey = host + "\n" + remoteDir;
    if (targets[targetKey]) {
      throw new Error("host 与 remote_dir 组合不能重复：" + name);
    }
    names[name] = true;
    targets[targetKey] = true;
    return { name: name, host: host, remote_dir: remoteDir, config_index: index };
  });

  const maxSize = environment("max_size_mb") || "100";
  const maxSafeMiB = "8796093022207";
  if (
    !/^[1-9][0-9]*$/.test(maxSize) ||
    maxSize.length > maxSafeMiB.length ||
    (maxSize.length === maxSafeMiB.length && maxSize > maxSafeMiB)
  ) {
    throw new Error("max_size_mb 必须是不会导致大小计算溢出的正整数");
  }

  return normalized;
}

function sortByRecentUse(hosts) {
  const recent = readRecentHosts();
  const positions = Object.create(null);
  recent.forEach(function (item, index) {
    positions[item.host + "\n" + item.remote_dir] = index;
  });

  return hosts.slice().sort(function (left, right) {
    const leftKey = left.host + "\n" + left.remote_dir;
    const rightKey = right.host + "\n" + right.remote_dir;
    const leftRecent = Object.prototype.hasOwnProperty.call(positions, leftKey)
      ? positions[leftKey]
      : Number.MAX_SAFE_INTEGER;
    const rightRecent = Object.prototype.hasOwnProperty.call(positions, rightKey)
      ? positions[rightKey]
      : Number.MAX_SAFE_INTEGER;
    if (leftRecent !== rightRecent) {
      return leftRecent - rightRecent;
    }
    return left.config_index - right.config_index;
  });
}

function errorResult(message) {
  return JSON.stringify({
    items: [
      {
        title: "主机配置错误",
        subtitle: message,
        valid: false
      }
    ]
  });
}

function listHosts(query) {
  let hosts;
  try {
    hosts = sortByRecentUse(validateConfig());
  } catch (error) {
    return errorResult(error.message || String(error));
  }

  const normalizedQuery = (query || "").trim().toLowerCase();
  if (normalizedQuery) {
    hosts = hosts.filter(function (item) {
      return (
        item.name.toLowerCase().indexOf(normalizedQuery) !== -1 ||
        item.host.toLowerCase().indexOf(normalizedQuery) !== -1
      );
    });
  }

  if (hosts.length === 0) {
    return JSON.stringify({
      items: [
        {
          title: "没有匹配的远端主机",
          subtitle: "请修改搜索词或检查 hosts_json",
          valid: false
        }
      ]
    });
  }

  return JSON.stringify({
    items: hosts.map(function (item) {
      return {
        title: item.name,
        subtitle: item.host + " → " + item.remote_dir,
        arg: item.name,
        match: item.name + " " + item.host,
        valid: true,
        variables: {
          upload_name: item.name,
          upload_host: item.host,
          upload_remote_dir: item.remote_dir
        }
      };
    })
  });
}

function markUsed(host, remoteDir) {
  if (!host || !remoteDir) {
    throw new Error("mark-used 缺少 host 或 remote_dir");
  }
  const recent = readRecentHosts().filter(function (item) {
    return item.host !== host || item.remote_dir !== remoteDir;
  });
  recent.unshift({ host: host, remote_dir: remoteDir });
  writeRecentHosts(recent);
  return "";
}

function run(argv) {
  const command = argv.length > 0 ? argv[0] : "list";
  if (command === "list") {
    return listHosts(argv.length > 1 ? argv[1] : "");
  }
  if (command === "mark-used") {
    return markUsed(argv[1], argv[2]);
  }
  throw new Error("unknown command: " + command);
}
