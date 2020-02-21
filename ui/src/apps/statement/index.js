module.exports = {
  id: 'statement',
  loader: () => import('./app.js'),
  routerPrefix: '/statement',
  icon: 'line-chart',
  menuKey: 'statement.nav_menu',
  translations: {
    en: require('./translations/en.yaml'),
    zh_CN: require('./translations/zh_CN.yaml'),
  },
}
