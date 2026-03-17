(function () {
  "use strict";

  /**
   * Detect the user's operating system from browser signals.
   * Returns "macos", "windows", "linux", or "unknown".
   */
  function detectOS() {
    var ua = navigator.userAgent || "";
    var platform = navigator.platform || "";

    // navigator.userAgentData is available in Chromium-based browsers
    if (navigator.userAgentData && navigator.userAgentData.platform) {
      var p = navigator.userAgentData.platform.toLowerCase();
      if (p === "macos") return "macos";
      if (p === "windows") return "windows";
      if (p === "linux") return "linux";
    }

    // Fallback to userAgent / platform sniffing
    if (/Mac/i.test(platform) || /Mac OS/i.test(ua)) return "macos";
    if (/Win/i.test(platform) || /Windows/i.test(ua)) return "windows";
    if (/Linux/i.test(platform) || /Linux/i.test(ua)) return "linux";

    return "unknown";
  }

  var os = detectOS();
  var names = { macos: "macOS", windows: "Windows", linux: "Linux", unknown: "your OS" };

  // Update detected OS text
  var detectEl = document.getElementById("detected-os");
  if (detectEl) {
    detectEl.textContent = names[os] || "your OS";
  }

  var cards = {
    macos: document.getElementById("card-macos"),
    linux: document.getElementById("card-linux"),
    windows: document.getElementById("card-windows"),
  };

  var tabs = {
    macos: document.getElementById("show-macos"),
    linux: document.getElementById("show-linux"),
    windows: document.getElementById("show-windows"),
  };

  /**
   * Show the card for the given OS and update tab visibility.
   */
  function showPlatform(target) {
    // Hide all cards
    Object.keys(cards).forEach(function (key) {
      if (cards[key]) cards[key].hidden = true;
    });
    // Show target card
    if (cards[target]) cards[target].hidden = false;

    // Show tabs for other platforms
    Object.keys(tabs).forEach(function (key) {
      if (tabs[key]) tabs[key].hidden = key === target;
    });
  }

  // Wire up tab buttons
  Object.keys(tabs).forEach(function (key) {
    if (tabs[key]) {
      tabs[key].addEventListener("click", function () {
        showPlatform(key);
      });
    }
  });

  // Show detected platform on load, or all if unknown
  if (os !== "unknown") {
    showPlatform(os);
  } else {
    Object.keys(cards).forEach(function (key) {
      if (cards[key]) cards[key].hidden = false;
    });
    Object.keys(tabs).forEach(function (key) {
      if (tabs[key]) tabs[key].hidden = true;
    });
  }
})();
