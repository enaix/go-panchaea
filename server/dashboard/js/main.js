var app = new Vue({
  el: '#dashboard',
  data: {
    status: '...',
    statusWrapperColor: 'c11-bg c0-fg',
    isWarningsCollapsed: true,
    isErrorsCollapsed: true,
    warnings: [],
    warningsCount: 0,
    errors: [],
  }
})
