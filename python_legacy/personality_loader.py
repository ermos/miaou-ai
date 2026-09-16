import re
from pathlib import Path
from typing import Dict, List
from config import PERSONALITY_FILE

class PersonalityConfig:
    def __init__(self, config_file=None):
        if config_file is None:
            config_file = PERSONALITY_FILE
        
        self.config_file = Path(config_file)
        self.personality = {}
        self.rules = {}
        self.examples = {}
        self.system_prompt = ""
        self.questions = []
        
        self.load()
    
    def load(self):
        """Charge PERSONALITY.md et parse les sections"""
        if not self.config_file.exists():
            print(f"⚠️  PERSONALITY.md not found at {self.config_file}")
            self.set_defaults()
            return
        
        try:
            with open(self.config_file, 'r', encoding='utf-8') as f:
                content = f.read()
            
            # Parser les sections
            self.parse_personality(content)
            self.parse_rules(content)
            self.parse_examples(content)
            self.parse_system_prompt(content)
            self.parse_questions(content)
            print(f"✅ Personality loaded from {self.config_file}")
        
        except Exception as e:
            print(f"❌ Error loading personality: {e}")
            self.set_defaults()
    
    def parse_personality(self, content):
        """Extrait la section Core Personality"""
        match = re.search(
            r'## 🎯 Core Personality\n(.*?)\n## ',
            content,
            re.DOTALL
        )
        if match:
            self.personality = match.group(1).strip()
    
    def parse_rules(self, content):
        """Extrait les Behavioral Rules"""
        # Rule 1: English Preference
        rule1 = self.extract_section(content, 'Rule 1: English Preference')
        self.rules['english_preference'] = rule1
        
        # Rule 2: Pronunciation
        rule2 = self.extract_section(content, 'Rule 2: Pronunciation')
        self.rules['pronunciation'] = rule2
        
        # Rule 3: Engage Questions
        rule3 = self.extract_section(content, 'Rule 3: Engage')
        self.rules['engagement'] = rule3
        
        # Rule 4: Conversation Flow
        rule4 = self.extract_section(content, 'Rule 4: Conversational Flow')
        self.rules['conversation'] = rule4
    
    def extract_section(self, content, heading):
        """Extrait une section par heading"""
        # Cherche le heading et récupère le contenu jusqu'au prochain heading
        pattern = rf'#### {heading}.*?\n(.*?)(?=####|###|##|$)'
        match = re.search(pattern, content, re.DOTALL)
        if match:
            text = match.group(1).strip()
            # Nettoyer le texte (enlever les blocs de code, etc.)
            return text[:500]  # Limiter à 500 chars
        return ""
    
    def parse_examples(self, content):
        """Extrait les exemples de comportement"""
        match = re.search(
            r'## 📝 Examples by Scenario\n(.*?)(?=## |$)',
            content,
            re.DOTALL
        )
        if match:
            self.examples = match.group(1).strip()
    
    def parse_system_prompt(self, content):
        """Extrait le System Prompt"""
        match = re.search(
            r'## 🧠 System Prompt Injection.*?```\n(.*?)\n```',
            content,
            re.DOTALL
        )
        if match:
            self.system_prompt = match.group(1).strip()
    
    def parse_questions(self, content):
        """Extrait les questions du Question Pool"""
        match = re.search(
            r'## 📋 Question Pool.*?\n(.*?)(?=## |```|$)',
            content,
            re.DOTALL
        )
        if match:
            text = match.group(1)
            # Extraire les listes de questions
            questions = re.findall(r'-\s+"([^"]+)"', text)
            self.questions = questions if questions else []
    
    def get_system_prompt(self):
        """Retourne le prompt système complet"""
        return self.system_prompt
    
    def get_rule(self, rule_name):
        """Retourne une règle spécifique"""
        return self.rules.get(rule_name, "")
    
    def get_random_question(self):
        """Retourne une question aléatoire"""
        if self.questions:
            import random
            return random.choice(self.questions)
        return "How was your day? 😊"
    
    def set_defaults(self):
        """Fallback defaults si fichier manquant"""
        self.system_prompt = """You are Miaou, a friendly English learning chat buddy for children.

PERSONALITY:
- Always kind, encouraging, and supportive
- Never criticize - always be positive
- Patient and fun
- Show genuine interest in the child

LANGUAGE RULES:
1. ALWAYS encourage English. If user speaks French, gently ask them to say it in English
2. Provide translation help: "In French you said 'X', in English we say 'Y'"
3. Ask them to repeat in English

PRONUNCIATION & CORRECTION:
1. If you detect a speech recognition error, gently ask for clarification
2. If you detect a pronunciation issue, give a gentle tip: "Good try! The pronunciation is like this: ..."
3. Always be encouraging about mistakes

ENGAGEMENT:
1. Ask questions to help them talk about themselves
2. Ask "how was your day?", "what did you do?", "tell me about..."
3. Follow up with WHY/HOW/TELL ME MORE
4. Show genuine interest

CONVERSATION:
- Keep responses short and natural (2-3 sentences max usually)
- Use emojis to make it fun
- Ask one question at a time
- Don't lecture - be conversational"""
    
    def reload(self):
        """Recharge la config (utile pour live editing)"""
        self.load()
        print("🔄 Personality config reloaded")
