import { test } from 'node:test';
import assert from 'node:assert/strict';
import { resolveLanguage, translate } from './i18n.js';
test('system language detection and override', () => {
 for (const lang of ['zh-CN','zh-Hant-TW','zh_HK','zh']) assert.equal(resolveLanguage('auto',[lang]),'zh');
 for (const lang of ['en-US','fr-FR','ja-JP']) assert.equal(resolveLanguage('auto',[lang]),'en');
 assert.equal(resolveLanguage('en',['zh-CN']),'en');
 assert.equal(resolveLanguage('zh',['en-US']),'zh');
 assert.equal(resolveLanguage('auto',[]),'en');
});
test('translate visible strings without changing persisted values', () => {
 assert.equal(translate('开始监控','en'),'Start monitoring');
 assert.equal(translate('开始监控','zh'),'开始监控');
 assert.equal(translate('iPhone Duo 256GB 夜空色','en'),'iPhone Duo 256GB Night Sky');
 assert.equal(translate('Apple Store Fifth Avenue','en'),'Apple Store Fifth Avenue');
 assert.equal(translate(null,'en'),null);
 assert.equal(translate('全部门店 · 12','en'),'All stores · 12');
});
