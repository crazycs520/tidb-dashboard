module.exports = {
  id: 'keyvis',
  loader: () => import('./app.js'),
  routerPrefix: '/keyvis',
  icon: 'eye',
  menuKey: 'keyvis.nav_menu',
  isDefaultRouter: true,
  translations: {
    en: require('./translations/en.yaml'),
    zh_CN: require('./translations/zh_CN.yaml'),
  },
};
