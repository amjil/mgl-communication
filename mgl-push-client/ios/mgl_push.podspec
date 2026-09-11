#
# mgl_push
#
Pod::Spec.new do |s|
  s.name             = 'mgl_push'
  s.version          = '0.1.0'
  s.summary          = 'mgl-push Flutter client plugin'
  s.description      = 'Cross-platform push client for mgl-push'
  s.homepage         = 'https://github.com/amjil/mgl-communication'
  s.license          = { :type => 'MIT' }
  s.author           = { 'amjil' => 'dev@amjil.net' }
  s.source           = { :path => '.' }
  s.source_files     = 'Classes/**/*'
  s.dependency 'Flutter'
  s.platform = :ios, '13.0'
  s.frameworks = 'PushKit', 'UserNotifications'
  s.pod_target_xcconfig = { 'DEFINES_MODULE' => 'YES' }
  s.swift_version = '5.0'
end
