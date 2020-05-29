var app = new Vue({
  el: '#dashboard',
  data: {
    status: '...',
    statusWrapperColor: 'c11-bg c0-fg',
    isWarningsCollapsed: true,
    isErrorsCollapsed: true,
    warnings: ['[*] Cannot find config file panchaea-client.json', '[1] Thread 3 is stuck', '[1] Thread 2 is stuck'],
    warningsCount: 3,
    errors: ['[!] WorkUnit not found', '[3] Failed to reload WU'],
    errorsCount: 2,
  },
  methods: {
    isEven: function (a) {
      return a % 2 == 0
    },
    newWarning: function (warn) {
      this.warnings.push(warn)
      this.warningsCount = this.warnings.length
    }
  }
})
