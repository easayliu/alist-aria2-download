#!/usr/bin/env python3
"""
Telegram Handler 重构助手脚本

此脚本帮助自动化 telegram.go 文件的重构过程，
提取函数和生成新的文件结构。
"""

import os
import re
import sys
from pathlib import Path
from typing import List, Dict, Tuple

class TelegramRefactorHelper:
    def __init__(self, source_file: str, target_dir: str):
        self.source_file = source_file
        self.target_dir = Path(target_dir)
        self.functions = {}
        self.constants = {}
        self.types = {}
        
    def analyze_source_file(self):
        """分析源文件，提取函数、常量和类型定义"""
        print(f"📖 分析源文件: {self.source_file}")
        
        with open(self.source_file, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # 提取函数定义
        self._extract_functions(content)
        
        # 提取常量
        self._extract_constants(content)
        
        # 提取类型定义
        self._extract_types(content)
        
        print(f"✅ 分析完成: 找到 {len(self.functions)} 个函数")
        
    def _extract_functions(self, content: str):
        """提取函数定义"""
        # 匹配Go函数定义的正则表达式
        function_pattern = r'func\s+(\([^)]*\))?\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\([^)]*\)([^{]*)?{'
        
        matches = re.finditer(function_pattern, content)
        for match in matches:
            receiver = match.group(1) or ""
            name = match.group(2)
            
            # 提取完整的函数体
            start_pos = match.start()
            brace_count = 0
            pos = match.end() - 1  # 从第一个 { 开始
            
            while pos < len(content):
                if content[pos] == '{':
                    brace_count += 1
                elif content[pos] == '}':
                    brace_count -= 1
                    if brace_count == 0:
                        break
                pos += 1
            
            if brace_count == 0:
                function_body = content[start_pos:pos+1]
                self.functions[name] = {
                    'receiver': receiver.strip(),
                    'body': function_body,
                    'category': self._categorize_function(name)
                }
    
    def _extract_constants(self, content: str):
        """提取常量定义"""
        # 匹配const定义
        const_pattern = r'const\s+(\w+)\s*=\s*([^/\n]+)'
        matches = re.findall(const_pattern, content)
        
        for name, value in matches:
            self.constants[name] = value.strip()
    
    def _extract_types(self, content: str):
        """提取类型定义"""
        # 匹配type定义
        type_pattern = r'type\s+(\w+)\s+struct\s*{[^}]*}'
        matches = re.findall(type_pattern, content, re.DOTALL)
        
        for match in matches:
            # 这里需要更复杂的解析逻辑
            pass
    
    def _categorize_function(self, name: str) -> str:
        """根据函数名称分类函数"""
        categories = {
            'command': ['handle.*Command', 'handle(Start|Help|Download|List|Cancel|Tasks|AddTask|QuickTask|DelTask|RunTask)'],
            'callback': ['handle.*Callback', 'handle.*WithEdit', 'handleCallbackQuery'],
            'render': ['render.*', 'get.*Keyboard', 'get.*Menu'],
            'util': ['format.*', 'escape.*', 'split.*', 'encode.*', 'decode.*'],
            'message': ['send.*', 'edit.*', 'answer.*'],
            'file': ['handleFile.*', 'handleBrowse.*', 'handleDownloadFile.*'],
            'task': ['handleTask.*', 'handleQuick.*', 'handleAdd.*', 'handleDel.*', 'handleRun.*'],
            'system': ['handleSystem.*', 'handleHealth.*', 'handleAlist.*'],
            'manual': ['handleManual.*', 'parseTime.*', 'callManual.*'],
        }
        
        for category, patterns in categories.items():
            for pattern in patterns:
                if re.match(pattern, name):
                    return category
        
        return 'other'
    
    def generate_file_structure(self):
        """生成新的文件结构"""
        print("🏗️  生成新的文件结构...")
        
        # 创建目录结构
        directories = [
            'interfaces', 'core', 'commands', 'callbacks', 
            'renderers', 'utils', 'config', 'types'
        ]
        
        for dir_name in directories:
            dir_path = self.target_dir / dir_name
            dir_path.mkdir(parents=True, exist_ok=True)
            print(f"   📁 创建目录: {dir_path}")
        
        # 生成文件映射
        file_mapping = self._get_file_mapping()
        
        # 生成各个文件
        for file_path, functions in file_mapping.items():
            self._generate_file(file_path, functions)
    
    def _get_file_mapping(self) -> Dict[str, List[str]]:
        """获取文件映射关系"""
        mapping = {
            'commands/base.go': [],
            'commands/download.go': [],
            'commands/file.go': [],
            'commands/task.go': [],
            'commands/system.go': [],
            'commands/help.go': [],
            'callbacks/base.go': [],
            'callbacks/menu.go': [],
            'callbacks/file_ops.go': [],
            'callbacks/download_ops.go': [],
            'callbacks/preview.go': [],
            'utils/formatter.go': [],
            'utils/message_sender.go': [],
            'utils/validator.go': [],
            'utils/encoder.go': [],
        }
        
        # 根据分类将函数分配到对应文件
        for func_name, func_info in self.functions.items():
            category = func_info['category']
            
            if category == 'command':
                if 'download' in func_name.lower():
                    mapping['commands/download.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['file', 'browse', 'list']):
                    mapping['commands/file.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['task', 'quick', 'add', 'del', 'run']):
                    mapping['commands/task.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['system', 'health', 'alist']):
                    mapping['commands/system.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['start', 'help']):
                    mapping['commands/help.go'].append(func_name)
                else:
                    mapping['commands/base.go'].append(func_name)
            
            elif category == 'callback':
                if 'menu' in func_name.lower():
                    mapping['callbacks/menu.go'].append(func_name)
                elif 'file' in func_name.lower():
                    mapping['callbacks/file_ops.go'].append(func_name)
                elif 'download' in func_name.lower():
                    mapping['callbacks/download_ops.go'].append(func_name)
                elif 'preview' in func_name.lower() or 'manual' in func_name.lower():
                    mapping['callbacks/preview.go'].append(func_name)
                else:
                    mapping['callbacks/base.go'].append(func_name)
            
            elif category == 'util':
                if any(word in func_name.lower() for word in ['format', 'escape', 'split']):
                    mapping['utils/formatter.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['send', 'edit', 'answer']):
                    mapping['utils/message_sender.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['encode', 'decode', 'path']):
                    mapping['utils/encoder.go'].append(func_name)
                elif any(word in func_name.lower() for word in ['parse', 'valid']):
                    mapping['utils/validator.go'].append(func_name)
        
        return mapping
    
    def _generate_file(self, relative_path: str, function_names: List[str]):
        """生成单个文件"""
        if not function_names:
            return
            
        file_path = self.target_dir / relative_path
        package_name = relative_path.split('/')[0]
        
        print(f"   📄 生成文件: {file_path}")
        
        # 生成文件头部
        content = f"""package {package_name}

// 此文件由重构脚本自动生成
// 源文件: {self.source_file}

import (
	// TODO: 添加必要的导入
)

"""
        
        # 添加函数
        for func_name in function_names:
            if func_name in self.functions:
                func_info = self.functions[func_name]
                content += f"// {func_name} - 从原文件迁移\n"
                content += func_info['body'] + "\n\n"
        
        # 写入文件
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
    
    def generate_summary_report(self):
        """生成重构摘要报告"""
        report_path = self.target_dir / "REFACTORING_SUMMARY.md"
        
        print(f"📊 生成重构报告: {report_path}")
        
        content = f"""# Telegram Handler 重构摘要

## 原始文件分析
- 源文件: `{self.source_file}`
- 函数总数: {len(self.functions)}
- 常量总数: {len(self.constants)}

## 函数分类统计
"""
        
        # 统计各类别函数数量
        categories = {}
        for func_info in self.functions.values():
            category = func_info['category']
            categories[category] = categories.get(category, 0) + 1
        
        for category, count in sorted(categories.items()):
            content += f"- {category}: {count} 个函数\n"
        
        content += "\n## 重构后文件结构\n\n"
        content += "```\n"
        content += "telegram/\n"
        
        for dir_name in ['interfaces', 'core', 'commands', 'callbacks', 'renderers', 'utils', 'config', 'types']:
            content += f"├── {dir_name}/\n"
            dir_path = self.target_dir / dir_name
            if dir_path.exists():
                for file_path in sorted(dir_path.glob("*.go")):
                    content += f"│   ├── {file_path.name}\n"
        
        content += "```\n\n"
        
        content += """## 下一步行动

1. ✅ 已完成结构化提取
2. 🔄 需要手动调整导入依赖
3. 🔄 需要实现接口定义
4. 🔄 需要编写单元测试
5. 🔄 需要集成测试验证

## 注意事项

- 生成的文件需要手动调整导入语句
- 函数间的依赖关系需要重新整理
- 建议逐步迁移，保持原文件作为备份
"""
        
        with open(report_path, 'w', encoding='utf-8') as f:
            f.write(content)

def main():
    """主函数"""
    if len(sys.argv) != 3:
        print("用法: python3 refactor_telegram.py <源文件路径> <目标目录>")
        print("示例: python3 refactor_telegram.py internal/api/handlers/telegram.go internal/api/handlers/telegram/")
        sys.exit(1)
    
    source_file = sys.argv[1]
    target_dir = sys.argv[2]
    
    if not os.path.exists(source_file):
        print(f"❌ 源文件不存在: {source_file}")
        sys.exit(1)
    
    print("🚀 开始Telegram Handler重构...")
    print(f"📁 源文件: {source_file}")
    print(f"📁 目标目录: {target_dir}")
    print()
    
    helper = TelegramRefactorHelper(source_file, target_dir)
    
    try:
        # 分析源文件
        helper.analyze_source_file()
        print()
        
        # 生成文件结构
        helper.generate_file_structure()
        print()
        
        # 生成摘要报告
        helper.generate_summary_report()
        print()
        
        print("✅ 重构助手执行完成!")
        print()
        print("📋 下一步:")
        print("1. 检查生成的文件并调整导入语句")
        print("2. 实现接口定义")
        print("3. 编写单元测试")
        print("4. 逐步替换原有实现")
        print("5. 运行集成测试验证功能")
        
    except Exception as e:
        print(f"❌ 执行过程中发生错误: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()